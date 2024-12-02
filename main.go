package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"

	"bytes"

	"gitlab.com/gomidi/midi/v2"

	"gitlab.com/gomidi/midi/v2/smf"

	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv" // autoregisters driver
	"path"
)

const (
	MML_COMPILER              = "sakuramml"
	DEFAULT_INCLUDE_FILE_NAME = "include"
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
}

type ToPlayerConfig interface {
	PlayerConfig() MmlMidiPlayerConfig
}

type MmlModule struct {
	includeFiles IncludeFiles
	mmlFiles     MmlFiles
}

func NewMmlModule(includeFiles IncludeFiles, mmlFiles MmlFiles) MmlModule {
	firstMmlFileDir := path.Dir(string(mmlFiles[0]))
	defaultIncludeFile := NewCleanPath(path.Join(firstMmlFileDir, DEFAULT_INCLUDE_FILE_NAME))

	includeFiles = append(includeFiles, defaultIncludeFile)

	return MmlModule{
		includeFiles: includeFiles,
		mmlFiles:     mmlFiles,
	}
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
			mmlModule:   NewMmlModule(includeFiles, mmlFiles),
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
	}

	return config
}

func InitCli() CliArgs {
	cliArgs := ParseCliArgs()

	if cliArgs.help {
		flag.Usage()
	}

	return cliArgs
}

func main() {
	var config ToPlayerConfig = InitCli()

	var mmlMidiPlayerConfig MmlMidiPlayerConfig = config.PlayerConfig()

	for _, mmlModuleMidiOutPortMap := range mmlMidiPlayerConfig.mmlModuleMidiOutPortMaps {
		go func() {

			var (
				mmlModule   = mmlModuleMidiOutPortMap.mmlModule
				midiOutPort = mmlModuleMidiOutPortMap.midiOutPort
			)

			smfFilePath := CompileMml(mmlModule)

			data, err := os.ReadFile(string(smfFilePath))
			if err != nil {
				log.Fatal(err)
			}
			SendMidiMessage(midiOutPort, data)

		}()
	}
}

func CompileMml(mmlModule MmlModule) CleanPath {
	smfFilePath := CreateTempSmfFile()
	mmlFilePath := SaveTempMmlFile(ConcatMmlModule(mmlModule))

	cmd := exec.Command(MML_COMPILER, string(mmlFilePath), "-o", string(smfFilePath))

	if err := cmd.Run(); err != nil {
		log.Printf("Error: %v", err)
	}

	return smfFilePath
}

func ConcatMmlModule(mmlModule MmlModule) string {
	var mmlCode string

	includeMmlPaths := []CleanPath{}

	for _, includeFile := range mmlModule.includeFiles {
		if includeFileData, err := os.ReadFile(string(includeFile)); err != nil {
			for _, line := range bytes.Split(includeFileData, []byte("\n")) {
				path := NewCleanPath(string(bytes.TrimSpace(line)))
				includeMmlPaths = append(includeMmlPaths, path)
			}
		}
	}

	for _, includeMmlPath := range includeMmlPaths {
		if includeMmlData, err := os.ReadFile(string(includeMmlPath)); err != nil {
			mmlCode += string(includeMmlData)
		}
	}

	for _, mmlFile := range mmlModule.mmlFiles {
		if mmldata, err := os.ReadFile(string(mmlFile)); err != nil {
			mmlCode += string(mmldata)
		}
	}

	return mmlCode
}

func SaveTempMmlFile(mmlCode string) CleanPath {
	tmpfile, err := os.CreateTemp("", "mml-*.mml")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := tmpfile.Write([]byte(mmlCode)); err != nil {
		log.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatal(err)
	}
	return NewCleanPath(tmpfile.Name())
}

func CreateTempSmfFile() CleanPath {
	tmpfile, err := os.CreateTemp("", "mml-*.mid")
	if err != nil {
		log.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatal(err)
	}
	return NewCleanPath(tmpfile.Name())
}

func SendMidiMessage(midiPort string, smfData []byte) {
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
		// log.Printf("track %v @%vms %s\n", ev.TrackNo, ev.AbsMicroSeconds/1000, ev.Message)
	}).Play(out)
}
