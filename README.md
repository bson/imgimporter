# imgimporter
This is a very simple utility to import raw Nikon NEF images to a Lightroom-like directory tree.  I created
it to simplify image import when Adobe stopped supporting Lightroom 6, to use with DxO PhotoLab 2.

It takes two arguments: a mounted volume like an SD/XQD card, and a directory where you keep images.  These
default to /Volumes/"NIKON Z7  " and ~/Pictures since I'm an OS X user.

It will scan the media files, using their EXIF creation date to determine where to copy them to, in the
format ~/Pictures/2019/2019-05-28.  The file type (extension) determines a further subdirectory: RAWS, JPEGS, or MOVIES.
This permits easily editing only one kind in DXO PL2 while shooting all three (e.g. RAW+JPEG and the occasional
movie).  I edit movies with Final Cut Pro, so it's nice to have clean folders with only movie files.  It recognizes
.NEF, .CRW, CRS, and .DNG as raw formats, .JPG and .JPEG as JPEG formats, and .MOV as movies.

It won't copy over existing files.  This makes it easy for example when traveling to copy images to a computer but
leave them on the cards so there are two copies.  At home the computer is backed up to a NAS and the cards can be
reformatted.  So each run incrementally copies the images from the last run.

It's probably possible to use the creation time of files instead of the EXIF origin datetime.  That would be much faster
since for large cards with a huge number of files yet a small incremental set to copy almost the entire time is spent
scanning media files for EXIF info.  It would also permit copying over certain other sidecar files some cameras produce
that don't have EXIF.
