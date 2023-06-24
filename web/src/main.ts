import {Bee} from "@ethersphere/bee-js";
import videojs from "video.js";

const bee = new Bee(window.location.origin)
const owner =  "5BDAB2F5bC2C311D1E5860617eE0ECC238f4933E"
const topic = "4f8faa5a6c4176b5d08e4721617aeafdca927e6c79ffb33f1d67cebb5546136c"

let player = videojs("radio-player")
let indexHint = 0

async function latestM3u8ManifestURL () {

    const start = Date.now();

    let reader = bee.makeFeedReader("sequence", topic, owner)
    let socRef = await reader.download()

    const end = Date.now();

    console.log("latest", socRef.reference, `execution time: ${end - start} ms`)

    return "/bytes/" + socRef.reference
}

async function asd()  {
    let url = await latestM3u8ManifestURL()

    // every n seconds, fetch the swarm refence url for the lastest the M3u8 manifest 
    setInterval(async () => {
        url = await latestM3u8ManifestURL()
    }, 10_000)

    
    const globalRequestHook =  (options) => {

        console.log(options)

        // adds the swarm reference url for the latest M3u8 manifest
        if (options.timeout > 0) {
            options.uri = url
        } else {
            options.uri = options.uri
        }

        return options
    };
    
    (videojs as any).Vhs.xhr.beforeRequest = globalRequestHook;
    
    player.src({
        src: url,
        type: 'application/x-mpegURL',
     });

     player.poster("https://media4.giphy.com/media/v1.Y2lkPTc5MGI3NjExaG9vaHBnazE5dXY3eG1hbzd6bWt3dmhsNmRrcWd0YmtlajR4ZGg5ZiZlcD12MV9pbnRlcm5hbF9naWZfYnlfaWQmY3Q9Zw/MwHRlY9M6GT8T7a3vN/giphy.gif")

     player.play()

}

asd()

