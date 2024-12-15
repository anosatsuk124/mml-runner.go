package mml

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"path"

	"github.com/anosatsuk124/mml-runner/packages/common"
)

const (
	MML_COMPILER              = "sakuramml"
	DEFAULT_INCLUDE_FILE_NAME = "includes"
)

type MmlFile struct {
	Path         common.CleanPath
	IsExecutable bool
}

type MmlFiles = []MmlFile

type IncludeFiles = common.CleanPathSlice

type ExecutableFiles = common.CleanPathSlice

type MmlModuleMidiOutPortMap struct {
	MidiOutPort string
	MmlModule   MmlModule
}

type MmlMidiPlayerConfig struct {
	MmlModuleMidiOutPortMaps []MmlModuleMidiOutPortMap
}

type MmlModule struct {
	IncludeFiles IncludeFiles
	MmlFiles     MmlFiles
}

func NewMmlModule(includeFiles IncludeFiles, mmlFiles MmlFiles) MmlModule {
	if len(mmlFiles) > 0 {
		firstMmlFileDir := path.Dir(string(mmlFiles[0].Path))
		defaultIncludeFile := common.NewCleanPath(path.Join(firstMmlFileDir, DEFAULT_INCLUDE_FILE_NAME))
		includeFiles = append(includeFiles, defaultIncludeFile)
	}

	return MmlModule{
		IncludeFiles: includeFiles,
		MmlFiles:     mmlFiles,
	}
}

func CompileMml(mmlModule MmlModule) common.CleanPath {
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

func ExecutableFileToMmlString(execFile common.CleanPath) string {
	var mmlCode string

	output, err := exec.Command(string(execFile)).Output()

	if err != nil {
		log.Println("err: ", err)
	}

	mmlCode += string(output) + "\n"

	return mmlCode
}

func ConcatMmlModule(mmlModule MmlModule) string {
	var mmlCode string

	includeMmlPaths := []common.CleanPath{}

	for _, includeFile := range mmlModule.IncludeFiles {
		if includeFileData, err := os.ReadFile(string(includeFile)); err == nil {
			for _, line := range bytes.Split(includeFileData, []byte("\n")) {
				path := common.NewCleanPath(string(bytes.TrimSpace(line)))
				includeMmlPaths = append(includeMmlPaths, path)
			}
		}
	}

	for _, includeMmlPath := range includeMmlPaths {
		if includeMmlData, err := os.ReadFile(string(includeMmlPath)); err == nil {
			mmlCode += string(includeMmlData)
		}
	}

	for _, mmlFile := range mmlModule.MmlFiles {
		if mmlFile.IsExecutable {
			mmlCode += ExecutableFileToMmlString(mmlFile.Path)
		} else if mmldata, err := os.ReadFile(string(mmlFile.Path)); err == nil {
			mmlCode += string(mmldata)
		}
	}

	return mmlCode
}

func SaveTempMmlFile(mmlCode string) common.CleanPath {
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
	return common.NewCleanPath(tmpfile.Name())
}

func CreateTempSmfFile() common.CleanPath {
	tmpfile, err := os.CreateTemp("", "mml-*.mid")
	if err != nil {
		log.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatal(err)
	}
	return common.NewCleanPath(tmpfile.Name())
}
