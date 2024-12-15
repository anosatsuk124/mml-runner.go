# mml-runner

## Installation

```bash
go install github.com/anosatsuk124/mml-runner/packages/mml-runner@0.1.4
```

## Usage

```bash
mml-runner -i <Include file (it expands into the head of mml files)> -f <MML file> -p <Midi Out port> -e <Executable file>

Usage of mml-runner:
  -e value
    	Executable files to execute and expand the output as MML (Optional)
  -f value
    	MML files to process (Required)
  -h	Show help
  -i value
    	Include files to process (Optional)
  -p string
    	Midi Out port to use (Required)
```

## License Information

```
   Copyright 2024 Satsuki Akiba <anosatsuk124@gmail.com>

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
```
