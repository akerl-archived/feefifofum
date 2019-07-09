feefifofum
=========

[![Build Status](https://img.shields.io/travis/com/akerl/feefifofum.svg)](https://travis-ci.com/akerl/feefifofum)
[![GitHub release](https://img.shields.io/github/release/akerl/feefifofum.svg)](https://github.com/akerl/feefifofum/releases)
[![MIT Licensed](https://img.shields.io/badge/license-MIT-green.svg)](https://tldrlegal.com/license/mit-license)

AWS Lambda that provides simple FIFO queus for HTTP requests. POSTs are stored, and then returned for matching GET requests.

## Usage

## Installation

The methods below describe how to create a payload.zip that can be used for AWS Lambdas.

### Official build process

This requires that you have Docker installed and running. It will launch a Docker b
uild container, build the binary, and create a zip file for loading into AWS Lambda
. The zip file can be found at `./pkg/payload.zip`.

```
make
```

### Local pkgforge build

This doesn't require Docker but does require that you have [the pkgforge gem](https://github.com/akerl/pkgforge) installed. It builds a zip file at `./pkg/payload.zip`

```
pkgforge build
```

### Local manual build

This method has no deps other than golang, make, and zip. You have to manually create the zip file.

```
make local
cp ./bin/feefifofum_linux ./main
zip payload.zip ./main
```

## License

feefifofum is released under the MIT License. See the bundled LICENSE file for details.
