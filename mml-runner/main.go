package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"bytes"

	"gitlab.com/gomidi/midi/v2"

	"github.com/fsnotify/fsnotify"
	"gitlab.com/gomidi/midi/v2/drivers"
	"gitlab.com/gomidi/midi/v2/smf"

	"path"

	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv" // autoregisters driver
)

const (
	MML_COMPILER              = "sakuramml"
	DEFAULT_INCLUDE_FILE_NAME = "includes"
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
	if len(mmlFiles) > 0 {
		firstMmlFileDir := path.Dir(string(mmlFiles[0]))
		defaultIncludeFile := NewCleanPath(path.Join(firstMmlFileDir, DEFAULT_INCLUDE_FILE_NAME))
		includeFiles = append(includeFiles, defaultIncludeFile)
	}

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
		if !(len(mmlFiles) > 0) || midiOutPort == "" {
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

		os.Exit(1)
	}

	return cliArgs
}

func play(quitChan chan struct{}, mmlMidiPlayerConfig MmlMidiPlayerConfig) {
	for _, mmlModuleMidiOutPortMap := range mmlMidiPlayerConfig.mmlModuleMidiOutPortMaps {
		var (
			mmlModule   = mmlModuleMidiOutPortMap.mmlModule
			midiOutPort = mmlModuleMidiOutPortMap.midiOutPort
		)

		smfFilePath := CompileMml(mmlModule)

		data, err := os.ReadFile(string(smfFilePath))
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Available MIDI OutPorts:\n" + midi.GetOutPorts().String() + "\n")
		out, err := midi.FindOutPort(midiOutPort)
		if err != nil {
			log.Fatal("Midi Out Port not found: ", midiOutPort)
			return
		}

		SendMidiMessage(quitChan, out, data)
	}
}

func main() {
	var config ToPlayerConfig = InitCli()

	var mmlMidiPlayerConfig MmlMidiPlayerConfig = config.PlayerConfig()

	defer midi.CloseDriver()

	quitChan := make(chan struct{})
	signalChan := make(chan os.Signal, 2)

	// Handle SIGINT
	signal.Notify(signalChan, os.Interrupt)

	var (
		mmlFiles        = mmlMidiPlayerConfig.mmlModuleMidiOutPortMaps[0].mmlModule.mmlFiles
		firstMmlFileDir = path.Dir(string(mmlFiles[0]))
	)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()
	log.Println("Watching: ", firstMmlFileDir)

	err = watcher.Add(firstMmlFileDir)
	if err != nil {
		log.Fatal(err)
	}

	go play(quitChan, mmlMidiPlayerConfig)

	for {
		select {
		case event, ok := <-watcher.Events:
			log.Println("File Changed: ", event.Name)
			if !ok {
				log.Println("watcher.Events is not ok")
				return
			}
			quitChan <- struct{}{}
			time.Sleep(100 * time.Millisecond)
			go play(quitChan, mmlMidiPlayerConfig)

		case err, ok := <-watcher.Errors:
			if !ok {
				log.Println("watcher.Errors is not ok")
				return
			}
			log.Println("error:", err)

		case <-signalChan:
			log.Println("Signal Received")
			quitChan <- struct{}{}
			log.Println("Shutting Down")
			time.Sleep(100 * time.Millisecond)
			return
		}
	}
}

func CompileMml(mmlModule MmlModule) CleanPath {
	smfFilePath := CreateTempSmfFile()
	log.Println("smfFilePath: ", smfFilePath)

	mmlFilePath := SaveTempMmlFile(ConcatMmlModule(mmlModule))
	log.Println("mmlFilePath: ", mmlFilePath)

	cmd := exec.Command(MML_COMPILER, string(mmlFilePath), string(smfFilePath))

	if output, err := cmd.Output(); err != nil {
		log.Printf("Error: %v", err)
	} else {
		log.Println(string(output))
	}

	return smfFilePath
}

func ConcatMmlModule(mmlModule MmlModule) string {
	var mmlCode string

	includeMmlPaths := []CleanPath{}

	for _, includeFile := range mmlModule.includeFiles {
		if includeFileData, err := os.ReadFile(string(includeFile)); err == nil {
			for _, line := range bytes.Split(includeFileData, []byte("\n")) {
				path := NewCleanPath(string(bytes.TrimSpace(line)))
				includeMmlPaths = append(includeMmlPaths, path)
			}
		}
	}

	for _, includeMmlPath := range includeMmlPaths {
		if includeMmlData, err := os.ReadFile(string(includeMmlPath)); err == nil {
			mmlCode += string(includeMmlData)
		}
	}

	for _, mmlFile := range mmlModule.mmlFiles {
		if mmldata, err := os.ReadFile(string(mmlFile)); err == nil {
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

func AllNoteOff(out drivers.Out) {
	for ch := 0; ch < 16; ch++ {
		for n := 0; n < 128; n++ {
			note := midi.NoteOff(uint8(ch), uint8(n))
			out.Send(note)
		}
	}
}

func SendMidiMessage(quitChan chan struct{}, out drivers.Out, smfData []byte) {
	if !out.IsOpen() {
		err := out.Open()
		if err != nil {
			log.Fatal(err)
		}
	}

	rd := bytes.NewReader(smfData)

	go func() {
		// read and play it
		smf.ReadTracksFrom(rd).Do(func(ev smf.TrackEvent) {
			log.Printf("track %v @%vms %s\n", ev.TrackNo, ev.AbsMicroSeconds/1000, ev.Message)
		}).Play(out)
	}()

	for {
		select {
		case <-quitChan:
			log.Println("All Note Off")
			AllNoteOff(out)
			log.Println("Midi Out Port Closing")
			err := out.Close()
			log.Println("Midi Out Port Closed")
			if err != nil {
				log.Fatal(err)
			}
			return
		}
	}
}
