


## Step 1

Upload the feed with an owner's address and topic hash

ex: 

```
export BATCH=**
export OWNER=**
export TOPIC=4f8faa5a6c4176b5d08e4721617aeafdca927e6c79ffb33f1d67cebb5546136c
```

`curl -XPOST -H "Swarm-Postage-Batch-Id: $BATCH" localhost:1633/feeds/$OWNER/$TOPIC`

## Step 2

Run the upload server

```
cd swarm-radio-server
go run main.go --batch-id= --private-key=
```

## Step 3

Run the ffmepg command to upload data from an mp3

```
ffmpeg -re -i skrillex.mp3 -f hls -hls_time 30 -hls_list_size 5 -hls_flags independent_segments -method POST http://localhost:9999/live/out.m3u8
```

## Step 4

Run the web app

```
cd swarw-radio-server/web
npm run build && cp ./dist/bundle.js ./public/bundle.js
```