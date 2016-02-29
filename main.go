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
	flag.StringVar(&address, "address", "11407", "address where to listen")
	flag.Parse()

	var authenticator auth.AuthenticatorInterface
	if htpasswdFn != ""	{
		log.Printf( "using htpassswd fn: %s \n", htpasswdFn )
		pwd, _ := os.Getwd()
		htpasswdFn = path.Join(pwd, htpasswdFn)
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
		url := videoInfos[0].YTWatchUrl
		req.URL.RawQuery = url
		log.Printf("lucky url was: %s", url)
		youtubeEndpoint(w, req)
	}
}

func youtubeEndpoint(w http.ResponseWriter, req *http.Request) {
	url := req.URL.RawQuery
	_ = url
	// var err error
	if mp3File, err := fetchYoutubeVideoToMp3File(url); err != nil {
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
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}
func fetchYoutubeVideo(url string) (string, error) {
	// youtube-dl -t --extract-audio --audio-format=mp3 https://www.youtube.com/watch?v=NUsoVlDFqZg
	return execCmdPipeStderr(exec.Command("youtube-dl", "-t", "--extract-audio", "--audio-format=mp3", url))
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
				return nil, fmt.Errorf("unable to get thumb for video. %s", err)
			}else 	{
				// fmt.Printf( "class: %#v\n", node.Attributes()["class"].String() )
				srcUrl.Host = "youtube.com"
				srcUrl.Scheme = "http"
				
				srcs = append(srcs, VideoInfo{
					Title: node.InnerHtml(),
					YTWatchUrl: srcUrl.String(),
					ThumbUrl: thumbNode[0].String(), 
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

func fetchYoutubeVideoToMp3File(url string) (filePath string, err error) {
	if out, err := fetchYoutubeVideo(url); err != nil {
		return "", err
	} else if matches := destRe.FindStringSubmatch(out); matches == nil || len(matches) != 2 {
		log.Printf("matches: %#v", matches)
		return "", fmt.Errorf("destination file filePath could not be parsed:\n%s", out)
	} else {
		// filePath = path.Join(musicTopDir, matches[1])
		filePath := matches[1]
		if exists, err := exists(filePath); err != nil {
			return "", fmt.Errorf("internal error: %s ", err)
		} else if !exists {
			log.Printf("mp3 file was not created at: %s", filePath)
			cwd, _ := os.Getwd()
			log.Printf("cwd: %s ", cwd)
			return "", fmt.Errorf("mp3 file was not created at: %s", filePath)
		} else {
			return filePath, nil
		}
	}
}

type VideoInfo struct {
	Title      string
	YTWatchUrl string //eg "watch?v=0IL0a3DgNtA"
	ThumbUrl string
}

// https://www.youtube.com/watch?v=0SkZxQZwFAM
var youtubeURLPrefix = "https://www.youtube.com"


var musicDowloaderListParser = regexp.MustCompile("(?m)^(.*)\t(.*)$")

/*func listVideoUrls(query string) ([]VideoInfo, error) {
	if out, err := execCmdPipeStderr(exec.Command("music_downloader.py", "-L", query)); err != nil {
		log.Printf("out/err was: %s\n", out)
		return nil, fmt.Errorf("error running music_downloader.py:\n%s\n%s\n", err, out)
	} else {
		results := musicDowloaderListParser.FindAllStringSubmatch(out, -1)
		infos := make([]VideoInfo, len(results))
		for i, line := range results {
			infos[i] = VideoInfo{
				Title:      line[1],
				YTWatchUrl: line[2],
			}
		}
		return infos, nil
	}
}*/

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
		html += fmt.Sprintf(
			`<tr><td><a href="%s">%s</a></td><td><img src="%s"></td></tr>`,
			localFetchEndpoint(video.YTWatchUrl),
			video.Title,
			video.ThumbUrl, 
		)
		html += "\n"
	}
	html += "</table>"
	return html
}

func localFetchEndpoint(url string) string {
	return "/youtube?" + url
}

// exists returns whether the given file or directory exists or not
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
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
