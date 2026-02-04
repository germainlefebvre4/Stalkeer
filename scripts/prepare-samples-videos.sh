#!/bin/bash

SCRIPT_DIR=$(readlink -f `dirname "$0"`)
ROOT_DIR=$(readlink -f "$SCRIPT_DIR/..")

# echo $SCRIPT_DIR
# echo $ROOT_DIR

M3U_SOURCE="m3u_playlist/sample_template.m3u"
M3U_DEST="m3u_playlist/sample.m3u"
cp $M3U_SOURCE $M3U_DEST

# Check m3u source file exists
if [ ! -f $M3U_SOURCE ] ; then
  echo " M3U source file $M3U_SOURCE does not exist"
  exit 1
fi

# Check sample files exist
if [ ! -f $ROOT_DIR/webserver/html/samples/sample_FRENCH_720p.mkv ] || [ ! -f $ROOT_DIR/webserver/html/samples/sample_FRENCH_720p.mp4 ] ; then
  echo " Sample files do not exist in webserver/html/samples/"
  echo " Please run 'make download-sample-videos' first"
  exit 1
fi

echo " Create directories in webserver/html/"
while read line; do
  echo "  Creating directory webserver/html/$line"
  mkdir -p $ROOT_DIR/webserver/html/$line;
done < <(grep '^http' $M3U_DEST | cut -d '/' -f 4- | grep -e 'mp4' -e '.mkv' -e '.avi' -e '.flv' | rev | cut -d '/' -f 2- | rev | sort -u)

count_mkv=0
echo " Create symlinks for .mkv files (expected 2min)"
while read line; do
  if [ ! -L $ROOT_DIR/webserver/html/$line ]; then
    # Generate path prefix
    num_slash=$(echo "$line" | awk -F"/" '{print NF-1}') # Number of slashes
    path_prefix=$(printf '../%.0s' $(seq 1 $num_slash)) # Generate ../ for each slash
    path_prefix=${path_prefix%/} # Remove last /
    # Create symlink
    ln -s $path_prefix/samples/sample_FRENCH_720p.mkv $ROOT_DIR/webserver/html/$line;
    count_mkv=$((count_mkv + 1))
  fi
done < <(grep '^http' $M3U_DEST | cut -d '/' -f 4- | grep -e '.mkv')
echo " Created $count_mkv symlinks for .mkv files"

count_mp4=0
echo " Create symlinks for .mp4 files (expected 30s)"
while read line; do
  if [ ! -L $ROOT_DIR/webserver/html/$line ]; then
    # Generate path prefix
    num_slash=$(echo "$line" | awk -F"/" '{print NF-1}') # Number of slashes
    path_prefix=$(printf '../%.0s' $(seq 1 $num_slash)) # Generate ../ for each slash
    path_prefix=${path_prefix%/} # Remove last /
    # Create symlink
    ln -s $path_prefix/samples/sample_FRENCH_720p.mp4 $ROOT_DIR/webserver/html/$line;
    count_mp4=$((count_mp4 + 1))
  fi
done < <(grep '^http' $M3U_DEST | cut -d '/' -f 4- | grep -e '.mp4')
echo " Created $count_mp4 symlinks for .mp4 files"

echo " Change links to local webserver in m3u file"
cat $M3U_DEST | grep '^http' | cut -d '/' -f 1-3 | sort -u | while read line; do
  echo "  Replacing $line by http://localhost:8008"
  sed -i "s|$line/|http://localhost:8008/|g" $M3U_DEST
done
echo " Links updated in m3u file"
