package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"bytes"

	"gitlab.com/gomidi/midi/v2"

	"gitlab.com/gomidi/midi/v2/smf"

	"path"
	// _ "gitlab.com/gomidi/midi/v2/drivers/portmididrv" // autoregisters driver
	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv" // autoregisters driver
)

type CleanPath string

func NewCleanPath(p string) CleanPath {
	return CleanPath(path.Clean(p))
}

type CleanPathSlice []CleanPath

func (c *CleanPathSlice) String() string {
	return fmt.Sprintf("%v", *c)
}

func (c *CleanPathSlice) Set(v string) error {
	*c = append(*c, NewCleanPath(v))
	return nil
}

type MmlFiles = CleanPathSlice

type IncludeFiles = CleanPathSlice

type MmlModuleMidiOutPortMap struct {
	midiOutPort string
	mmlModule   MmlModule
}

type MmlMidiPlayerConfig struct {
	mmlModuleMidiOutPortMaps []MmlModuleMidiOutPortMap
	defaultIncludeFiles      IncludeFiles
}

type ToPlayerConfig interface {
	PlayerConfig() MmlMidiPlayerConfig
}

type MmlModule struct {
	includeFiles IncludeFiles
	mmlFiles     MmlFiles
}

// Represents the command line arguments
type CliArgs struct {
	mmlModuleMidiOutPortMap MmlModuleMidiOutPortMap
	help                    bool
}

// Parse the command line arguments
func ParseCliArgs() CliArgs {
	var (
		mmlFiles     MmlFiles
		includeFiles IncludeFiles
		midiOutPort  string
		help         bool
	)

	flag.StringVar(&midiOutPort, "p", "", "Midi Out port to use (Required)")
	flag.Var(&mmlFiles, "f", "MML file to process (Required)")
	flag.Var(&includeFiles, "i", "Include file to process (Optional)")
	flag.BoolVar(&help, "h", false, "Show help")

	{
		flag.Parse()
		if !(len(mmlFiles) > 0) {
			help = true
		}
	}

	return CliArgs{
		mmlModuleMidiOutPortMap: MmlModuleMidiOutPortMap{
			mmlModule: MmlModule{
				mmlFiles:     mmlFiles,
				includeFiles: includeFiles,
			},
			midiOutPort: midiOutPort,
		},
		help: help,
	}
}

func (c CliArgs) PlayerConfig() MmlMidiPlayerConfig {
	config := MmlMidiPlayerConfig{
		mmlModuleMidiOutPortMaps: []MmlModuleMidiOutPortMap{
			c.mmlModuleMidiOutPortMap,
		},
		defaultIncludeFiles: []CleanPath{NewCleanPath("include")},
	}
	return config
}

func main() {
	cliArgs := ParseCliArgs()

	if cliArgs.help {
		flag.Usage()
		return
	}

	var (
		mmlMidiPlayerConfig MmlMidiPlayerConfig
	)
	mmlMidiPlayerConfig = cliArgs.PlayerConfig()

	var (
		mmlFile     = mmlMidiPlayerConfig.mmlModuleMidiOutPortMaps[0].mmlModule.mmlFiles[0]
		midiOutPort = mmlMidiPlayerConfig.mmlModuleMidiOutPortMaps[0].midiOutPort
	)

	data, err := os.ReadFile(string(mmlFile))

	if err != nil {
		log.Fatal(err)
	}

	sendMidiMessage(midiOutPort, data)
}

func sendMidiMessage(midiPort string, smfData []byte) {
	defer midi.CloseDriver()

	fmt.Printf("Available MIDI OutPorts:\n" + midi.GetOutPorts().String() + "\n")

	out, err := midi.FindOutPort(midiPort)
	if err != nil {
		log.Fatal("Midi Out Port not found: ", midiPort)
		return
	}

	rd := bytes.NewReader(smfData)

	// read and play it
	smf.ReadTracksFrom(rd).Do(func(ev smf.TrackEvent) {
		log.Printf("track %v @%vms %s\n", ev.TrackNo, ev.AbsMicroSeconds/1000, ev.Message)
	}).Play(out)
}
