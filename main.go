package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var Version string

type keyValue struct {
	key   string
	value string
}

func main() {
	var ver bool
	var u string
	var f string
	var m string
	var t string
	var nrep uint64
	var auth string
	var bearer string
	var ctype string
	var outpath string
	var bodystr string
	var basicUser string
	var basicPass string
	flag.BoolVar(&ver, "v", false, "print version and exit")
	flag.StringVar(&u, "u", "", "request url (required)")
	flag.StringVar(&f, "f", "", "path to file with request body")
	flag.StringVar(&bodystr, "s", "", "specify request body as a string")
	flag.StringVar(&m, "m", "", "request method (GET by default, or QUERY when body specified)")
	flag.StringVar(&t, "t", "5s", "request timeout (set to empty for no timeout)")
	flag.StringVar(&auth, "a", "", "auth header")
	flag.StringVar(&bearer, "b", "", "bearer token in auth header")
	flag.StringVar(&outpath, "o", "", "path to output file (stdout by default)")
	flag.StringVar(&ctype, "p", "auto", "content type header")
	flag.Uint64Var(&nrep, "n", 0, "repeat request multiple times sequentially (1 + n total requests)")
	flag.StringVar(&basicUser, "basic-user", "", "username for basic auth")
	flag.StringVar(&basicPass, "basic-pass", "", "password for basic auth")

	var queryParams []keyValue
	flag.Func("q", "query parameter key=value pair, can be used multiple times", func(s string) error {
		key, value, ok := strings.Cut(s, "=")
		if !ok {
			return errors.New("query parameter must take form of key=value pair")
		}

		queryParams = append(queryParams, keyValue{
			key:   key,
			value: value,
		})
		return nil
	})

	var cookies []keyValue
	flag.Func("c", "request cookie key=value pair, can be used multiple times", func(s string) error {
		key, value, ok := strings.Cut(s, "=")
		if !ok {
			return errors.New("cookie must take form of key=value pair")
		}

		cookies = append(cookies, keyValue{
			key:   key,
			value: value,
		})
		return nil
	})

	var headers []keyValue
	flag.Func("x", "request header key=value pair, can be used multiple times", func(s string) error {
		key, value, ok := strings.Cut(s, "=")
		if !ok {
			return errors.New("header must take form of key=value pair")
		}

		headers = append(headers, keyValue{
			key:   key,
			value: value,
		})
		return nil
	})

	// TODO: flag for providing custom tls certificates

	flag.Parse()

	if ver {
		if Version == "" {
			Version = "dev"
		}
		fmt.Println(Version)
		return
	}

	u = strings.TrimSpace(u)
	if u == "" {
		fmt.Fprintf(os.Stderr, "[fatal] empty request url (-u flag)\n")
		fmt.Fprint(os.Stderr, `
Usage:
	
	hit [options] -u <url>  | send request
	hit -v                  | print version
	hit -h                  | help

`)
		flag.PrintDefaults()
		os.Exit(1)
	}

	auth = strings.TrimSpace(auth)
	bearer = strings.TrimSpace(bearer)
	if auth != "" && bearer != "" {
		fmt.Fprintln(os.Stderr, "flags -a and -b cannot be used together")
		os.Exit(1)
	}

	if f != "" && bodystr != "" {
		fmt.Fprintln(os.Stderr, "flags -f and -s cannot be used together")
		os.Exit(1)
	}

	var timeout time.Duration
	var err error

	t = strings.TrimSpace(t)
	switch t {
	case "", "0":
		// continue execution
	default:
		timeout, err = time.ParseDuration(t)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[fatal] parse timeout string: %v\n", err)
			os.Exit(1)
		}
	}
	if timeout < 0 {
		fmt.Fprintf(os.Stderr, "[fatal] invalid (negative) timeout\n")
		os.Exit(1)
	}

	m = strings.ToUpper(m)
	switch m {
	case "":
		if f == "" && bodystr == "" {
			m = "GET"
		} else {
			m = "QUERY"
		}
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete,
		http.MethodHead, http.MethodPatch, http.MethodConnect:
		// continue execution
	case "QUERY":
		// continue execution
	default:
		fmt.Fprintf(os.Stderr, "[warn] unknown \"%s\" request method\n", m)
	}

	if len(queryParams) != 0 {
		parsed, err := url.Parse(u)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[fatal] bad url \"%s\": %v\n", u, err)
			os.Exit(1)
		}

		q := parsed.Query()
		for _, pair := range queryParams {
			q.Add(pair.key, pair.value)
		}
		parsed.RawQuery = q.Encode()
		u = parsed.String()
	}

	var body io.Reader
	if f != "" {
		if ctype == "auto" {
			switch filepath.Ext(f) {
			case ".json":
				ctype = "application/json"
			case ".txt":
				ctype = "text/plain"
			default:
				ctype = ""
			}
		}

		// TODO: open and stream large files
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		body = bytes.NewReader(data)
		fmt.Printf("[info] read %d bytes from \"%s\" for request body\n\n", len(data), f)
	} else if bodystr != "" {
		body = strings.NewReader(bodystr)
		fmt.Printf("[info] take %d bytes from string for request body\n\n", len(bodystr))
	}
	if ctype == "auto" {
		ctype = ""
	}

	if body != nil && m == "GET" {
		fmt.Printf("[warn] GET request configured with body\n")
	}
	req, err := http.NewRequestWithContext(context.Background(), m, u, body)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	for _, h := range headers {
		req.Header.Add(h.key, h.value)
	}

	for _, c := range cookies {
		req.AddCookie(&http.Cookie{
			Name:  c.key,
			Value: c.value,
		})
	}

	if body != nil && ctype != "" {
		req.Header.Add("Content-Type", ctype)
	}

	if auth == "" && bearer != "" {
		auth = "Bearer " + bearer
	}
	if auth != "" {
		req.Header.Add("Authorization", auth)
	} else if basicUser != "" && basicPass != "" {
		req.SetBasicAuth(basicUser, basicPass)
	}

	var client http.Client
	client.Timeout = timeout

	fmt.Printf("===== REQUEST HEADERS =====\n\n")
	fmt.Printf("%s %s\n", req.Method, req.URL)
	printRequestHeaders(req)
	fmt.Println()

	start := time.Now()
	r, err := client.Do(req)
	helapsed := time.Since(start)

	fmt.Printf("===== STATS =====\n\n")
	fmt.Printf("elapsed: %s\n", helapsed.Round(10*time.Microsecond))
	fmt.Printf("clength: %s\n\n", formatContentLength(r.ContentLength))
	if err != nil {
		fmt.Printf("=============\n\n")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer r.Body.Close()

	fmt.Printf("===== RESPONSE HEADERS =====\n\n")
	fmt.Printf("Status: %s\n\n", r.Status)
	printResponseHeaders(r)
	fmt.Println()

	if r.ContentLength != 0 {
		var nread int64 // number of bytes read from body

		fmt.Printf("===== RESPONSE BODY =====\n\n")
		var belapsed time.Duration
		if strings.Contains(r.Header.Get("Content-Type"), "json") {
			nread, belapsed = printJSONBody(r.Body)
		} else {
			start := time.Now()
			nread, err = io.Copy(os.Stdout, r.Body)
			belapsed = time.Since(start)

			if err != nil {
				fmt.Fprintf(os.Stderr, "[error] read response body: %s\n", err)
			}
			io.WriteString(os.Stdout, "\n\n=============\n\n")
		}

		total := helapsed + belapsed
		fmt.Printf("elapsed: %v (%v + %v)\n", total.Round(100*time.Microsecond), helapsed.Round(10*time.Microsecond), belapsed.Round(10*time.Microsecond))
		fmt.Printf("body.read: %d bytes\n", nread)
	} else {
		fmt.Printf("elapsed: %v\n", helapsed.Round(10*time.Microsecond))
	}

	rcookies := r.Cookies()
	if len(rcookies) != 0 {
		fmt.Println()

		for _, c := range rcookies {
			fmt.Printf("cookie.set: %s=%s\n", c.Name, c.Value)
		}
	}
}

func printRequestHeaders(r *http.Request) {
	r.Header.Clone().Write(os.Stdout)
}

func printResponseHeaders(r *http.Response) {
	r.Header.Write(os.Stdout)
}

// returns number of bytes successfully read from body and time
// it took to read
func printJSONBody(body io.Reader) (int64, time.Duration) {
	var data bytes.Buffer
	start := time.Now()
	n, err := io.Copy(&data, body)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("[error] read response body: %s\n", err)
		return n, elapsed
	}
	buf := bytes.Buffer{}
	_ = json.Indent(&buf, data.Bytes(), "", "    ")
	_, _ = os.Stdout.Write(buf.Bytes())

	io.WriteString(os.Stdout, "\n\n=============\n\n")
	return n, elapsed
}

func formatContentLength(n int64) string {
	if n < 0 {
		return fmt.Sprintf("%d (unknown)", n)
	}
	if n < 1<<13 {
		return strconv.FormatInt(n, 10)
	}
	if n < 1<<23 {
		return fmt.Sprintf("%d (%d kb)", n, n/(1<<10))
	}
	return fmt.Sprintf("%d (%d mb)", n, n/(1<<20))
}
