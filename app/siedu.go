package app

import (
	"bookget/config"
	"bookget/lib/gohttp"
	"bookget/lib/util"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
)

type SiEdu struct {
	dt *DownloadTask
}

func (r *SiEdu) Init(iTask int, sUrl string) (msg string, err error) {
	r.dt = new(DownloadTask)
	r.dt.UrlParsed, err = url.Parse(sUrl)
	r.dt.Url = sUrl
	r.dt.Index = iTask
	r.dt.BookId = r.getBookId(r.dt.Url)
	if r.dt.BookId == "" {
		return "requested URL was not found.", err
	}
	r.dt.Jar, _ = cookiejar.New(nil)
	return r.download()
}

func (r *SiEdu) getBookId(sUrl string) (bookId string) {
	//https://ids.si.edu/ids/manifest/FS-F1904.61_006
	m := regexp.MustCompile(`manifest/([A-z0-9._-]+)`).FindStringSubmatch(sUrl)
	if m != nil {
		bookId = m[1]
	}
	if strings.Contains(sUrl, "/object/") {
		bs, err := r.getBody(sUrl, nil)
		if err != nil {
			return ""
		}
		//<div  data-idsid="FS-F1904.61_006" class="media-metadata">
		m = regexp.MustCompile(`data-idsid="([A-z0-9._-]+)"`).FindStringSubmatch(string(bs))
		if m != nil {
			bookId = m[1]
		}
	}
	return bookId
}

func (r *SiEdu) download() (msg string, err error) {
	name := util.GenNumberSorted(r.dt.Index)
	log.Printf("Get %s  %s\n", name, r.dt.Url)
	r.dt.SavePath = config.CreateDirectory(r.dt.Url, r.dt.BookId)
	apiUrl := "https://ids.si.edu/ids/manifest/" + r.dt.BookId
	canvases, err := r.getCanvases(apiUrl, r.dt.Jar)
	if err != nil || canvases == nil {
		return "requested URL was not found.", err
	}
	log.Printf(" %d images \n", len(canvases))
	return r.do(canvases)
}

func (r *SiEdu) do(iiifUrls []string) (msg string, err error) {
	if iiifUrls == nil {
		return
	}
	referer := url.QueryEscape(r.dt.Url)
	args := []string{"--dezoomer=iiif",
		"-H", "Origin:" + referer,
		"-H", "Referer:" + referer,
		"-H", "User-Agent:" + config.Conf.UserAgent,
	}
	size := len(iiifUrls)
	for i, uri := range iiifUrls {
		if uri == "" || !config.PageRange(i, size) {
			continue
		}
		sortId := util.GenNumberSorted(i + 1)
		log.Printf("Get %s  %s\n", sortId, uri)
		filename := sortId + config.Conf.FileExt
		inputUri := r.dt.SavePath + string(os.PathSeparator) + sortId + "_info.json"
		bs, err := r.getBody(uri, r.dt.Jar)
		if err != nil {
			continue
		}
		body := strings.Replace(string(bs), `"http://iiif.io/api/image/2/level2.json",`, "", -1)
		body = strings.Replace(body, `"sizeByH",`, "", -1)
		os.WriteFile(inputUri, []byte(body), os.ModePerm)

		dest := r.dt.SavePath + string(os.PathSeparator) + filename
		if ret := util.StartProcess(inputUri, dest, args); ret == true {
			os.Remove(inputUri)
		}
	}
	return "", nil
}

func (r *SiEdu) getVolumes(sUrl string, jar *cookiejar.Jar) (volumes []string, err error) {
	//TODO implement me
	panic("implement me")
}

func (r *SiEdu) getCanvases(sUrl string, jar *cookiejar.Jar) (canvases []string, err error) {
	bs, err := r.getBody(sUrl, jar)
	var manifest = new(ResponseManifest)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(bs, manifest); err != nil {
		log.Printf("json.Unmarshal failed: %s\n", err)
		return
	}
	if len(manifest.Sequences) == 0 {
		return
	}
	size := len(manifest.Sequences[0].Canvases)
	canvases = make([]string, 0, size)
	for _, canvase := range manifest.Sequences[0].Canvases {
		for _, image := range canvase.Images {
			iiiInfo := fmt.Sprintf("%s/info.json", image.Resource.Service.Id)
			canvases = append(canvases, iiiInfo)
		}
	}
	return canvases, nil
}

func (r *SiEdu) getBody(apiUrl string, jar *cookiejar.Jar) ([]byte, error) {
	cli := gohttp.NewClient(gohttp.Options{
		CookieFile: config.Conf.CookieFile,
		CookieJar:  jar,
		Headers: map[string]interface{}{
			"User-Agent": config.Conf.UserAgent,
			"Referer":    r.dt.Url,
			"authority":  "www.si.edu",
			"origin":     "https://www.si.edu/",
		},
	})
	resp, err := cli.Get(apiUrl)
	if err != nil {
		return nil, err
	}
	bs, _ := resp.GetBody()
	if resp.GetStatusCode() != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("ErrCode:%d, %s", resp.GetStatusCode(), resp.GetReasonPhrase()))
	}
	return bs, nil
}
