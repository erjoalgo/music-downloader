package main

import (
	// "strings"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"os/exec"
	"regexp"
	"github.com/moovweb/gokogiri"
	"github.com/abbot/go-http-auth"
	"flag"
)


func main() {
	
	var htpasswdFn string
	var musicTopDir string
	var address string
	flag.StringVar(&htpasswdFn, "htpasswd", "", "htpasswd file for basic auth")
	flag.StringVar(&musicTopDir, "top", "/tmp", "directory where downloads will go")
	flag.StringVar(&address, "address", ":11407", "address where to listen")
	flag.Parse()

	var authenticator auth.AuthenticatorInterface
	if htpasswdFn != ""	{
		log.Printf( "using htpassswd fn: %s \n", htpasswdFn )
		// what's the point of this
		// pwd, _ := os.Getwd()
		// htpasswdFn = path.Join(pwd, htpasswdFn)
		Secret := auth.HtpasswdFileProvider(htpasswdFn)
		authenticator = auth.NewBasicAuthenticator("localhost", Secret)
	}

	log.Printf( "using top dir: %s \n", musicTopDir)
	os.Chdir(musicTopDir)


	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		fmt.Fprintf(w, "OK")
	})

	mux.HandleFunc("/echo", func(w http.ResponseWriter, req *http.Request) {
		// The "/" pattern matches everything, so we need to check
		// that we're at the root here.
		fmt.Fprintf(w, "%#v", req.URL)
	})

	if authenticator != nil	{
		mux.HandleFunc("/youtube", auth.JustCheck(authenticator, youtubeEndpoint))
	}else 	{
		mux.HandleFunc("/youtube", youtubeEndpoint)
	}
	mux.HandleFunc("/lyrics", lyricsEndpoint)
	mux.HandleFunc("/lucky", luckyEndpoint)
	mux.HandleFunc("/prompt", promptEndpoint)
	mux.HandleFunc("/proxy", proxyEndpoint)
	mux.HandleFunc("/", promptEndpoint)

	log.Printf("listening on %s", address)
	log.Fatal(http.ListenAndServe(address, mux))
}

func lyricsEndpoint(w http.ResponseWriter, req *http.Request) {
	query := req.URL.RawQuery//the lyrics
	_url := queryToYtUrl(query)
	if html, err := downloadURL(_url); err != nil	{
		http.Error(w, fmt.Sprintf("error downloading %s, %s \n", _url, err), 500)
	}else if videoInfos, err := extractTitlesUrlsImages(html); err != nil	{
		http.Error(w, fmt.Sprintf("error extracting videos from %s, %s \n", _url, err), 500)
	}else 	{
		html := videoInfoListToHtml(videoInfos)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		fmt.Fprint(w, html)
	}
}

func promptEndpoint(w http.ResponseWriter, req *http.Request) {
	var html = `
<HTML>
<HEAD>
<TITLE>Music downloader</TITLE>
</HEAD>
<BODY>
<SCRIPT LANGUAGE="JAVASCRIPT" TYPE="TEXT/JAVASCRIPT">
<!--
query = window.prompt("Enter song search query", "bailando enrique");
// window.location = encodeURI("/lucky?"+query);
window.location = encodeURI("/lyrics?"+query);
//-->
</SCRIPT>
</BODY>
</HTML>

	`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	fmt.Fprintf(w, html)
}
func luckyEndpoint(w http.ResponseWriter, req *http.Request) {
	query := req.URL.RawQuery//query
	_url := queryToYtUrl(query)
	if html, err := downloadURL(_url); err != nil	{
		http.Error(w, fmt.Sprintf("error downloading %s, %s \n", _url, err), 500)
	}else if videoInfos, err := extractTitlesUrlsImages(html); err != nil	{
		http.Error(w, fmt.Sprintf("error extracting videos from %s, %s \n", _url, err), 500)
	}else {
		url := videoInfos[0].YTWatchUrl.String()
		req.URL.RawQuery = url
		log.Printf("lucky url was: %s", url)
		youtubeEndpoint(w, req)
	}
}

func youtubeEndpoint(w http.ResponseWriter, req *http.Request) {
	rawUrl := req.URL.RawQuery
	if _url, err := url.Parse(rawUrl); err != nil	{
		http.Error(w, fmt.Sprintf("error parsing url %s :%s", rawUrl, err), 500)
	}else if mp3File, err := fetchYoutubeVideoToMp3File(_url); err != nil {
		// if mp3File := "/home/ealfonso/repos/music-downloader/Juan Luis Guerra - Que me des tu cariño-oIuzP4nZRv4.mp3"; err != nil {
		http.Error(w, fmt.Sprintf("error fetching file:\n%s", err), 500)
	} else {
		log.Printf("serving mp3 file: %s", mp3File)
		// Content type: audio/mpeg
		// http://stackoverflow.com/questions/12017694/content-type-for-mp3-download-response
		// http.ServeFile(w, req, mp3File)
		// http.ServeFile(w, req, mp3File)
		if b, err := ioutil.ReadFile(mp3File); err != nil {
			http.Error(w, fmt.Sprintf("error opening file:\n%s", err), 500)
		} else {
			w.Header().Set("Content-Type", "audio/mpeg")
			w.WriteHeader(200)
			if _, err := bytes.NewBuffer(b).WriteTo(w); err != nil {
				http.Error(w, fmt.Sprintf("error writing file:\n%s", err), 500)
			}
		}
	}
}
func execCmdPipeStderr(cmd *exec.Cmd) (string, error) {
	// fmt.Printf( "running cmd: %s \n", cmd )
	//TODO this isn't workingft
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}
func fetchYoutubeVideo(url string) (string, error) {
	// youtube-dl -t --extract-audio --audio-format=mp3 https://www.youtube.com/watch?v=NUsoVlDFqZg
	return execCmdPipeStderr(exec.Command("youtube-dl", "--id", "--extract-audio", "--audio-format=mp3", url))
}

func downloadURL ( _url string) ([]byte, error)	{
	if _, err := url.Parse(_url); err != nil {
		return nil, fmt.Errorf("bad url %s: %s", _url, err)
	} else if response, err := http.Get(_url); err != nil {
		return nil, fmt.Errorf("open problem: %s", err)
	} else if html, err := ioutil.ReadAll(response.Body); err != nil {
		return nil, fmt.Errorf("read problem: %s", err)
	} else 	{
		return html, nil
	}
}


func extractTitlesUrlsImages(html []byte) ([]VideoInfo, error) {
	// imgXpath := `//a[contains(@class, 'yt-uix-tile-link')]`
	imgXpath := `//a[contains(@class, 'yt-uix-tile-link') and starts-with(@href,'/watch?v=')]`
	if doc, err := gokogiri.ParseHtml(html); err != nil {
		return nil, fmt.Errorf("parse problem: %s", err)
	} else if imgs, err := doc.Search(imgXpath); err != nil {
		return nil, fmt.Errorf("xpath problem: %s", err)
	} else {
		srcs := make([]VideoInfo, 0, 30)//usually 20
		for _, node := range imgs {
			_url := node.Attributes()["href"].String()
			if srcUrl, err := url.Parse(_url); err != nil {
				return nil, fmt.Errorf("bad url inside html: %#v %s", node, err)
			} else if thumbNode, err := node.Search("ancestor::li/div/div/div/a/div/span/img/@src"); err != nil	{
				/*note: only the visible thumbs on the first browser page are valid links (~1-6),
the rest are only activated when the user "scrolls down", which we can't easily emulate
fall back to thumbnail img.youtube.com url approach*/
				
				return nil, fmt.Errorf("unable to get thumb for video. %s", err)
			}else if thumbUrl, err := url.Parse(thumbNode[0].String()); err != nil	{
				return nil, fmt.Errorf("bad thumb url. %s", err)
			}else 	{
				srcUrl.Host = "youtube.com"
				srcUrl.Scheme = "http"

				thumbUrl.Scheme = "http"
				srcs = append(srcs, VideoInfo{
					Title: node.InnerHtml(),
					YTWatchUrl: srcUrl,
					ThumbUrl: thumbUrl, 
				})
			}
		}
		// imgXpath := `//div[@class="yt-thumb video-thumb"]/span/img`
		return srcs, nil
	}
}

// "[download] Destination: Juan Luis Guerra - Que me des tu cariño-oIuzP4nZRv4.m4a"
// [ffmpeg] Destination: Molotov - Puto-CzEbm7yup7g.mp3

var destRe = regexp.MustCompile("(?m)^[[]ffmpeg[]] Destination: (.*mp3)$")

func fetchYoutubeVideoToMp3File(_url *url.URL) (filePath string, err error) {
	if _id, err := youtubeUrlId(_url); err != nil	{
		return "", fmt.Errorf("destination file filePath could not be parsed:\n%s", err)
	} else if mp3Fn := _id+".mp3"; fileExists(mp3Fn)	{
		log.Printf( "cache hit for %s\n", mp3Fn )
		return mp3Fn, nil
	} else if out, err := fetchYoutubeVideo(_url.String()); err != nil {
		// return "", out+"\n"+err
		return "", fmt.Errorf("out: out\nerr: %s", out, err)
	} else if !fileExists(mp3Fn) {
		log.Printf("mp3 file was not created at: %s", mp3Fn)
		cwd, _ := os.Getwd()
		log.Printf("cwd: %s ", cwd)
		return "", fmt.Errorf("mp3 file was not created at: %s", mp3Fn)
	} else {
		return mp3Fn, nil
	}
}

type VideoInfo struct {
	Title      string
	YTWatchUrl *url.URL
	ThumbUrl *url.URL
}
func (v VideoInfo) ThumbUrlAuto ( i int ) 	string {
	//i must be 0-3
	// http://www.reelseo.com/youtube-thumbnail-image/
	// http://img.youtube.com/vi/bQVoAWSP7k4/0.jpg
	id := v.Id()
	return fmt.Sprintf("http://img.youtube.com/vi/%s/%d.jpg", id, i)
}
func (v VideoInfo) Id (  ) string	{
	id, _ := youtubeUrlId(v.YTWatchUrl);
	return id
}

func youtubeUrlId ( url *url.URL ) (string, error)	{
	//TODO errcheck
	return url.Query()["v"][0], nil
}
	
	
// https://www.youtube.com/watch?v=0SkZxQZwFAM


var musicDowloaderListParser = regexp.MustCompile("(?m)^(.*)\t(.*)$")

func queryToYtUrl ( query string ) string	{
	// return "https://www.youtube.com/results?search_query="+query//TODO
	base := "https://www.youtube.com/results?search_query="
	// fmt.Printf( "query: %s\n", query )
	// encoded := strings.Join(strings.Split(" ", query), "+")
	// _url := base+encoded
	// fmt.Printf( "%s\n", _url )
	return base+query
}

func videoInfoListToHtml(videos []VideoInfo) string {
	// <a href="http://www.w3schools.com">Visit W3Schools.com!</a>
	/*`<table style="width:100%">
	  <tr>
	    <td>Jill</td>
	    <td>Smith</td>
	    <td>50</td>
	  </tr>
	  <tr>
	    <td>Eve</td>
	    <td>Jackson</td>
	    <td>94</td>
	  </tr>
	</table> `*/

	var html = `<table class="fixed">`
	html += "\n"
	for _, video := range videos {
		_url := localFetchEndpoint(video.YTWatchUrl.String())
		html += fmt.Sprintf(`<tr>
<td><a href="%s"><img src="%s"></a></td>
<td><a href="%s">%s</a></td>
</tr>`,
			_url, 
			"/proxy?"+video.ThumbUrlAuto(0),
			_url, 
			video.Title,
			// "/proxy?"+video.ThumbUrl.String(), 
		)
		html += "\n"
	}
	html += "</table>"
	return html
}

func localFetchEndpoint(url string) string {
	return "/youtube?" + url
}

func fileExists(path string) (bool) {
	if _, err := os.Stat(path); err == nil	{
		//file exists
		return true
	}else if os.IsNotExist(err)	{
		//doesn't exist
		return false
	}else 	{
		// ??
		log.Fatal(err)
		panic(err)
	}
}

func proxyEndpoint(w http.ResponseWriter, req *http.Request) {
	_url := req.URL.RawQuery
	if response, err := http.Get(_url); err != nil {
		http.Error(w, fmt.Sprintf("error fetching url %s:\n%s", _url, err), 400)
	} else {
		w.WriteHeader(200)
		bodyReq, _ := ioutil.ReadAll(response.Body)
		bytes.NewBuffer(bodyReq).WriteTo(w)
		//this is actually slower
		// io.Copy(w, response.Body)
	}
}
