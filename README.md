# imgimporter

## About

This is a very simple utility to import raw Nikon NEF images to a Lightroom-like directory tree.  I created
it to simplify image import when Adobe stopped supporting Lightroom 6, to use with DxO PhotoLab 2.

It takes two arguments: a mounted volume like an SD/XQD card, and a directory where you keep images.
On MacOSX these default to /Volumes/"NIKON Z7  " and ~/Pictures.
On Windows they default to D: and %HOMEPATH%\Pictures.

It will scan the media files, using their EXIF creation date to determine where to copy them to, in the
format ~/Pictures/2019/2019-05-28.  The file type (extension) determines a further subdirectory: RAWS, JPEGS, or MOVIES.
This permits easily editing only one kind in DXO PL2 while shooting all three (e.g. RAW+JPEG and the occasional
movie).  I edit movies with Final Cut Pro, so it's nice to have clean folders with only movie files.  It recognizes
.NEF, .CRW, CRS, and .DNG as raw formats, .JPG and .JPEG as JPEG formats, and .MOV as movies.

It won't copy existing files.  This makes it easy for example when traveling to copy images to a computer but
leave them on the cards so there are two copies.  At home the computer is backed up to a NAS and the cards can be
reformatted.  So each run incrementally copies the images from the last run.

To speed up the scan for files to copy it checks the file modification time on the card, and if it exists already the
file is skipped.  If this differs from the Exif creation date it won't exist, and it retrieves the Exif time.  This is
much slower since it has to open and parse the media file, but in typical usage this shouldn't occur.  It might
occur for a file edited on the camera but I think this is a very atypical use case.

## Installation

Make sure you have GOPATH set up.  If you're new to go, check out any of the many intros out there, for
example https://golangbot.com/learn-golang-series/.  Then:
```
$ go install github.com/bson/imgimporter
```
This will download, build and install the code in this repo as $GOPATH/bin/imgimporter.
