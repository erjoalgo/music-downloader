package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
)

var musicTopDir = "/tmp" //TODO flags

func main() {
	// log.Fatal(destRe.FindStringSubmatch("[download] Destination: Juan Luis Guerra - Que me des tu cariño-oIuzP4nZRv4.m4a"))
	os.Chdir(musicTopDir)
	mux := http.NewServeMux()
	// mux.Handle("/api/", apiHandler{})

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

	mux.HandleFunc("/youtube", youtubeEndpoint)
	mux.HandleFunc("/lyrics", lyricsEndpoint)
	mux.HandleFunc("/lucky", luckyEndpoint)
	mux.HandleFunc("/prompt", promptEndpoint)
	mux.HandleFunc("/", promptEndpoint)

	log.Fatal(http.ListenAndServe(":11407", mux))
}

func lyricsEndpoint(w http.ResponseWriter, req *http.Request) {
	lyrics := req.URL.RawQuery
	if videoList, err := listVideoUrls(lyrics); err != nil {
		http.Error(w, fmt.Sprintf("error fetching video list:\n%s", err), 500)
	} else {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		fmt.Fprintf(w, videoInfoListToHtml(videoList))
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
	lyrics := req.URL.RawQuery
	if videoList, err := listVideoUrls(lyrics); err != nil {
		http.Error(w, fmt.Sprintf("error fetching video list:\n%s", err), 500)
	} else if len(videoList) == 0 {
		http.Error(w, fmt.Sprintf("no videos found for query: %s", lyrics), 400)
	} else {
		url := videoList[0].FullUrl()
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
		http.ServeFile(w, req, mp3File)
	}
}

func execCmdPipeStderr(cmd *exec.Cmd) (string, error) {
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
		filePath = path.Join(musicTopDir, matches[1])
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
}

// https://www.youtube.com/watch?v=0SkZxQZwFAM
var youtubeURLPrefix = "https://www.youtube.com"

func (v VideoInfo) FullUrl() string {
	return youtubeURLPrefix + v.YTWatchUrl
}

var musicDowloaderListParser = regexp.MustCompile("(?m)^(.*)\t(.*)$")

func listVideoUrls(query string) ([]VideoInfo, error) {
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

	var html = "<table>\n"
	for _, video := range videos {
		html += fmt.Sprintf(
			"<tr><td><a href=\"%s\">%s</a></td></tr>",
			localFetchEndpoint(video.FullUrl()),
			video.Title)
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
