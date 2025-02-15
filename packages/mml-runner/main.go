package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"time"

	"bytes"

	"gitlab.com/gomidi/midi/v2"

	"github.com/fsnotify/fsnotify"
	"gitlab.com/gomidi/midi/v2/drivers"
	"gitlab.com/gomidi/midi/v2/smf"

	"github.com/anosatsuk124/mml-runner/packages/common"
	"github.com/anosatsuk124/mml-runner/packages/mml"

	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv" // autoregisters driver
)

const (
	DEFAULT_WATING_TIME       = 1500 * time.Millisecond
	DEFAULT_FILE_REFRESH_TIME = 2500 * time.Millisecond
)

type ToPlayerConfig interface {
	PlayerConfig() mml.MmlMidiPlayerConfig
}

type MidiOutFile struct {
	IsSpecified bool
	Value       string
}

// Represents the command line arguments
type CliArgs struct {
	mmlModuleMidiOutPortMap mml.MmlModuleMidiOutPortMap
	help                    bool
	midiOutFile             MidiOutFile
	quiet                   bool
	once                    bool
}

// Parse the command line arguments
func ParseCliArgs() CliArgs {
	var (
		mmlFiles     mml.MmlFiles
		includeFiles mml.IncludeFiles
		midiOutPort  string
		midiOutFile  string
		quiet        bool
		once         bool
		help         bool
	)

	var (
		plainFiles             common.CleanPathSlice
		execFiles              common.CleanPathSlice
		midiOutFileIsSpecified bool = false
	)

	flag.StringVar(&midiOutPort, "p", "", "Midi Out port to use (Required)")
	flag.Var(&plainFiles, "f", "MML files to process (Required)")
	flag.Var(&includeFiles, "i", "Include files to process (Optional)")
	flag.Var(&execFiles, "e", "Executable files to execute and expand the output as MML (Optional)")
	flag.BoolVar(&help, "h", false, "Show help")
	flag.StringVar(&midiOutFile, "o", "", "Output file path (Optional)")
	flag.BoolVar(&quiet, "q", false, "Quiet compile only mode (Optional) (Output file path is required)")
	flag.BoolVar(&once, "1", false, "Run only once (Optional)")

	{
		flag.Parse()

		for _, plainFile := range plainFiles {
			mmlFiles = append(mmlFiles, mml.MmlFile{
				Path:         plainFile,
				IsExecutable: false,
			})
		}
		for _, execFile := range execFiles {
			mmlFiles = append(mmlFiles, mml.MmlFile{
				Path:         execFile,
				IsExecutable: true,
			})
		}

		if !(len(mmlFiles) > 0) {
			help = true
		}

		if midiOutPort == "" && midiOutFile == "" {
			help = true
		}

		if midiOutFile != "" {
			midiOutFileIsSpecified = true
		}
	}

	return CliArgs{
		mmlModuleMidiOutPortMap: mml.MmlModuleMidiOutPortMap{
			MmlModule:   mml.NewMmlModule(includeFiles, mmlFiles),
			MidiOutPort: midiOutPort,
		},
		help: help,
		midiOutFile: MidiOutFile{
			IsSpecified: midiOutFileIsSpecified,
			Value:       midiOutFile,
		},
		quiet: quiet,
		once:  once,
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

func play(mmlMidiPlayerConfig mml.MmlMidiPlayerConfig) {
	IsRunning = true

	var (
		mmlModuleMidiOutPortMaps = mmlMidiPlayerConfig.MmlModuleMidiOutPortMaps
		mmlModule                = mmlModuleMidiOutPortMaps[0].MmlModule
		midiOutPort              = mmlModuleMidiOutPortMaps[0].MidiOutPort
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

	go SendMidiMessage(out, data)

	select {
	case <-RestartChan:
		GracefulShutdown(out)
		return
	case <-SignalChan:
		GracefulShutdown(out)
		os.Exit(0)
	}
}

func GracefulShutdown(out drivers.Out) {
	AllSoundOff(out)
	if err := out.Close(); err != nil {
		log.Fatal(err)
	}
	log.Println("Midi Out Port Closed")

	time.Sleep(DEFAULT_WATING_TIME)

	IsRunning = false
}

func runQuiet(mmlMidiPlayerConfig mml.MmlMidiPlayerConfig) common.CleanPath {
	var (
		mmlModuleMidiOutPortMaps = mmlMidiPlayerConfig.MmlModuleMidiOutPortMaps
		mmlModule                = mmlModuleMidiOutPortMaps[0].MmlModule
	)

	smfFilePath := mml.CompileMml(mmlModule)

	return smfFilePath
}

func runOnce(mmlMidiPlayerConfig mml.MmlMidiPlayerConfig) {
	IsRunning = true

	var (
		mmlModuleMidiOutPortMaps = mmlMidiPlayerConfig.MmlModuleMidiOutPortMaps
		mmlModule                = mmlModuleMidiOutPortMaps[0].MmlModule
		midiOutPort              = mmlModuleMidiOutPortMaps[0].MidiOutPort
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

	SendMidiMessage(out, data)

	GracefulShutdown(out)
}

func runAndWatch(mmlMidiPlayerConfig mml.MmlMidiPlayerConfig) {
	// Handle SIGINT
	signal.Notify(SignalChan, os.Interrupt)

	var (
		mmlFiles = mmlMidiPlayerConfig.MmlModuleMidiOutPortMaps[0].MmlModule.MmlFiles
	)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	for _, file := range mmlFiles {
		dir := path.Dir(string(file.Path))
		err = watcher.Add(dir)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Watching: ", dir)
	}

	go func() {
		defer midi.CloseDriver()
		for {
			play(mmlMidiPlayerConfig)
		}
	}()

	var prevTime = time.Now()
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				log.Println("watcher.Events is not ok")
				return
			}
			if time.Since(prevTime) < DEFAULT_FILE_REFRESH_TIME {
				continue
			} else {
				prevTime = time.Now()
			}
			log.Println("File Changed: ", event.Name)
			RestartChan <- struct{}{}

		case err, ok := <-watcher.Errors:
			if !ok {
				log.Println("watcher.Errors is not ok")
				return
			}
			log.Println("error:", err)
		}
	}
}

var SignalChan = make(chan os.Signal, 2)
var RestartChan = make(chan struct{})
var IsRunning = false

func main() {
	var config = InitCli()

	var mmlMidiPlayerConfig mml.MmlMidiPlayerConfig = config.PlayerConfig()

	{
		tempSmfFilePath := runQuiet(mmlMidiPlayerConfig)
		if config.midiOutFile.IsSpecified {
			if err := os.Rename(string(tempSmfFilePath), config.midiOutFile.Value); err != nil {
				log.Fatal(err)
			}

			log.Println("Output file: ", config.midiOutFile.Value)
		}
	}

	if config.quiet {
		return
	}

	if config.once {
		runOnce(mmlMidiPlayerConfig)
	} else {
		runAndWatch(mmlMidiPlayerConfig)
	}
}

func AllSoundOff(out drivers.Out) {
	for ch := 0; ch < 16; ch++ {
		out.Send([]byte{0xB0 + byte(ch), byte(midi.AllSoundOff), 0})
	}
	log.Println("AllSoundOff")
}

func SendMidiMessage(out drivers.Out, smfData []byte) {
	rd := bytes.NewReader(smfData)

	// read and play it
	smf.ReadTracksFrom(rd).Do(func(ev smf.TrackEvent) {
		log.Printf("track %v @%vms %s\n", ev.TrackNo, ev.AbsMicroSeconds/1000, ev.Message)
	}).Play(out)
}
