# Swarm Radio

The goal of this project is to livestream media, music or videos, to Swarm, and have users access the media stream through a *single reference*.

To simplify the task, primary focus will be to stream live music to the network.

The reference should ideally point to a web app that hosts the radio, where the music automatically starts playing. 

### Tech

On the web side, we utilize the [HTML Live Streaming](https://en.wikipedia.org/wiki/HTTP_Live_Streaming) protocol, aka HLS.

The streaming starts with a `m3u8` manifest file which points to the sequences of downloadable media file URLs, seperated by timestamps.

For example: 
```
m3u8 #EXTM3U
#EXT-X-VERSION:6
#EXT-X-TARGETDURATION:30
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:30.003111,
stream/out0
#EXTINF:30.003111,
stream/out1
#EXT-X-ENDLIST
```

The `stream/out` entries are the patches of split media data, stored as seperate files, that the client can download and reconstruct the music, one patch at a time. 

The streamer has to decode the music into these patches, upload the patches to Swarm, and update the root m3u8 file so that the web client can know about the existence of the new patches, and reconstruct the live music accordingly.

Feeds is a powerful concept in Swarm, where a static reference points to the latest version of some mutable data.

The streamer then will maintain a feed that will always point to the latest version of the m3u8.

The client can then periodically query the feed to pick up the latest m3u8, to discover new patches of data, to continue playing live music.

The last missing piece of the puzzle is making the patches downloadable by the client via Swarm. To achieve this, the `out` file URLs from the example above have to be meaninful paths in the context of Swarm. As such, we replace the file URL with the Swarm reference, prepended by the download API endpoint.

```
m3u8 #EXTM3U
#EXT-X-VERSION:6
#EXT-X-TARGETDURATION:30
#EXT-X-MEDIA-SEQUENCE:0
#EXTINF:30.003111,
bytes/1b4asdasd0a416833fa1b9a7fef012b31663cde0d292d0933f77c77d1a08f
#EXTINF:30.003111,
bytes/39asdasdasd0098d290583b00030c932102cea9f1ac079d7939675ead6e19
#EXT-X-ENDLIST
```

## Streamer Architecture

Decoder (ffmpeg) -> Upload Server -> Local Swarm Node

We use ffmpeg to produce the HLS stream, which patch by patch, uploads the updated version of the m3u8 and patch data to the intermediary *Upload Server*, which then uploads the patch data to Swarm, and using the Swarm reference, replaces the local `out` file paths with the Swarm paths, and then finally updates the feed with the new m3u8.

## Next?

Swarm TV? Podcasts? 

## Demo

### Step 1

Upload the feed with an owner's address and topic hash.

The topic is the hash of the string: "test radio".

```
export BATCH=**
export OWNER=**
export TOPIC=4f8faa5a6c4176b5d08e4721617aeafdca927e6c79ffb33f1d67cebb5546136c

curl -XPOST -H "Swarm-Postage-Batch-Id: $BATCH" localhost:1633/feeds/$OWNER/$TOPIC`
```

### Step 2

Run the upload server

```
go run main.go --batch-id= --private-key=
```

### Step 3

Run the ffmepg command to upload data from an mp3

```
ffmpeg -re -i skrillex.mp3 -f hls -hls_time 30 -hls_list_size 5 -hls_flags independent_segments -method POST http://localhost:9999/live/out.m3u8
```

### Step 4

Build and Upload the web app

```
cd swarw-radio-server/web
npm run build && cp ./dist/bundle.js ./public/bundle.js

tar -cvf swarm-radio.tar -C dist/ .

curl \
    -X POST \
    -H "Content-Type: application/x-tar" \
    -H "Swarm-Index-Document: index.html" \
    -H "Swarm-Error-Document: error.html" \
    -H "Swarm-Collection: true" \
    -H "Swarm-Postage-Batch-Id: $BATCH" \
    --data-binary @swarm-radio.tar http://localhost:1633/bzz
```
