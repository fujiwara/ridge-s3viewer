package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dustin/go-humanize"

	"github.com/fujiwara/ridge"
)

var (
	BucketName     string
	WebSiteHosting bool
	HTTPS          bool
)

var html = `
<!doctype html>
<html charset="utf-8">
  <head>{{ $base := .BaseURL }}
    <title>{{ $bucket := .res.Name }}{{ $bucket }}/{{ .Prefix }}</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css" integrity="sha384-BVYiiSIFeK1dGmJRAkycuHAHRg32OmUcww7on3RYdg4Va+PmSTsz/K68vbdEjh4u" crossorigin="anonymous">
  </head>
  <body>
    <div class="container">
    <div class="page-header">
      <h1>s3://{{ $bucket }}/{{ .Prefix }}</h1>
    </div>
    <div class="list-group">
    {{if ne .Prefix "" }}
      <a href="../" class="list-group-item">
        <span class="glyphicon glyphicon-folder-close" aria-hidden="true"></span>
          ../
      </a>
    {{end}}
    {{range .res.CommonPrefixes}}
      <a href="{{ basename .Prefix }}/" class="list-group-item">
       <span class="glyphicon glyphicon-folder-close" aria-hidden="true"></span> {{ basename .Prefix }}/
      </a>
    {{end}}
    {{range .res.Contents}}
      {{if isDir .Key}}
      {{else}}
      <a href="{{$base}}/{{ .Key }}" class="list-group-item">
        <h4 class="list-group-item-heading">
          <span class="glyphicon glyphicon-file" aria-hidden="true"></span>
          {{ basename .Key }}
        </h4>
        <p class="list-group-item-text">{{ bytes .Size }} | {{ .LastModified }}</p>
      </a>
      {{end}}
    {{end}}
    </div>
    </div>
  </body>
</html>
`

func init() {
	BucketName = os.Getenv("BUCKET_NAME")
	WebSiteHosting, _ = strconv.ParseBool(os.Getenv("WEBSITE_HOSTING"))
	HTTPS, _ = strconv.ParseBool(os.Getenv("HTTPS"))
}

func main() {
	sess, err := session.NewSession()
	if err != nil {
		fmt.Println("failed to create session,", err)
		return
	}
	svc := s3.New(sess)
	tmpl := template.New("html")
	tmpl.Funcs(template.FuncMap{
		"hello": func(s string) string {
			return "Hello " + s
		},
		"isDir": func(s *string) bool {
			return strings.HasSuffix(*s, "/")
		},
		"bytes": func(s *int64) string {
			return humanize.Bytes(uint64(*s))
		},
		"basename": func(s *string) string {
			return path.Base(*s)
		},
	})
	if _, err := tmpl.Parse(html); err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		prefix := strings.TrimPrefix(r.URL.Path, "/")
		params := &s3.ListObjectsInput{
			Bucket:    aws.String(BucketName),
			Delimiter: aws.String("/"),
			Prefix:    aws.String(prefix),
		}
		resp, err := svc.ListObjects(params)
		if err != nil {
			log.Println("ListObjects failed", err)
			http.Error(w, fmt.Sprintf("%s", err), http.StatusInternalServerError)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		u := &url.URL{}
		if HTTPS {
			u.Scheme = "https"
		} else {
			u.Scheme = "http"
		}
		if WebSiteHosting {
			u.Host = BucketName
		} else {
			u.Host = BucketName + ".s3.amazonaws.com"
		}

		tmpl.ExecuteTemplate(w, "html", map[string]interface{}{
			"Prefix":  prefix,
			"res":     resp,
			"BaseURL": u.String(),
		})
	})

	ridge.Run(":8080", "/viewer", mux)
}
