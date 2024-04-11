# Cloudreve uploader

A simple command line tool to help upload files to cloudreve and get direct links. It helps typora to upload images to cloudreve and use cloudreve as a image server.

## Quick Start

#### pre-conditions

If you want to get a direct link to a file, you have to turn on the feature in the storage policy that allows for direct links to be obtained

#### Install:

If you have already installed golang

```sh
go install github.com/vincent-vinf/cloudreve-uploader
```

cloudreve-uploader will be installed on `$GOPATH/cloudreve-uploader ` or `$HOME/go/bin/cloudreve-uploader`(GOBIN environment variable not specified).

Alternatively, you can download the executable directly from the release.

#### Use in typora:

<img src="https://cloud.vinf.top/f/KEf4/image-20240411160227988.png" alt="image-20240411160227988" style="zoom:50%;" />

1. Enable the typora upload image function
2. Set command to the path where cloudreve-uploader is located

## Known Issues

* Only supports uploading files, not directories
* Tested on local storage policy only, other storage policies not yet tested
