package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"bytes"

	"gitlab.com/gomidi/midi/v2"

	"github.com/fsnotify/fsnotify"
	"gitlab.com/gomidi/midi/v2/drivers"
	"gitlab.com/gomidi/midi/v2/smf"

	"path"

	"github.com/anosatsuk124/mml-runner/packages/mml"

	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv" // autoregisters driver
)

type ToPlayerConfig interface {
	PlayerConfig() mml.MmlMidiPlayerConfig
}

// Represents the command line arguments
type CliArgs struct {
	mmlModuleMidiOutPortMap mml.MmlModuleMidiOutPortMap
	help                    bool
}

// Parse the command line arguments
func ParseCliArgs() CliArgs {
	var (
		mmlFiles     mml.MmlFiles
		includeFiles mml.IncludeFiles
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
		mmlModuleMidiOutPortMap: mml.MmlModuleMidiOutPortMap{
			MmlModule:   mml.NewMmlModule(includeFiles, mmlFiles),
			MidiOutPort: midiOutPort,
		},
		help: help,
	}
}

func (c CliArgs) PlayerConfig() mml.MmlMidiPlayerConfig {
	config := mml.MmlMidiPlayerConfig{
		MmlModuleMidiOutPortMaps: []mml.MmlModuleMidiOutPortMap{
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

func play(quitChan chan struct{}, mmlMidiPlayerConfig mml.MmlMidiPlayerConfig) {
	for _, mmlModuleMidiOutPortMap := range mmlMidiPlayerConfig.MmlModuleMidiOutPortMaps {
		var (
			mmlModule   = mmlModuleMidiOutPortMap.MmlModule
			midiOutPort = mmlModuleMidiOutPortMap.MidiOutPort
		)

		smfFilePath := mml.CompileMml(mmlModule)

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

	var mmlMidiPlayerConfig mml.MmlMidiPlayerConfig = config.PlayerConfig()

	defer midi.CloseDriver()

	quitChan := make(chan struct{})
	signalChan := make(chan os.Signal, 2)

	// Handle SIGINT
	signal.Notify(signalChan, os.Interrupt)

	var (
		mmlFiles        = mmlMidiPlayerConfig.MmlModuleMidiOutPortMaps[0].MmlModule.MmlFiles
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
