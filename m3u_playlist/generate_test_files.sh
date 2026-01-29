#!/bin/bash

# Script to generate large M3U test files

generate_m3u() {
    local count=$1
    local filename=$2
    
    echo "Generating $filename with $count entries..."
    
    {
        echo "#EXTM3U"
        for ((i=1; i<=count; i++)); do
            category=$((i % 10))
            case $category in
                0) group="Movies:Action" ;;
                1) group="Movies:Comedy" ;;
                2) group="Movies:Drama" ;;
                3) group="Movies:SciFi" ;;
                4) group="Movies:Horror" ;;
                5) group="Series:Drama" ;;
                6) group="Series:Comedy" ;;
                7) group="Series:SciFi" ;;
                8) group="Movies:Animation" ;;
                9) group="Movies:Documentary" ;;
            esac
            
            ext=$((i % 2))
            if [ $ext -eq 0 ]; then
                extension="mkv"
            else
                extension="mp4"
            fi
            
            echo "#EXTINF:-1 tvg-id=\"item$i\" tvg-name=\"Entry $i\" tvg-logo=\"http://example.com/posters/$i.jpg\" group-title=\"$group\",Entry $i"
            echo "http://example.com/media/item$i.$extension"
        done
    } > "$filename"
    
    echo "Generated $filename"
}

# Generate test files
generate_m3u 1000 "test_1000_entries.m3u"
generate_m3u 10000 "test_10000_entries.m3u"
generate_m3u 100000 "test_100000_entries.m3u"

echo "All test files generated successfully!"
