package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"

	"github.com/alecthomas/kingpin/v2"
	log "github.com/sirupsen/logrus"
)

var (
	verbose            = kingpin.Flag("verbose", "Verbose mode.").Short('v').Bool()
	upstream           = kingpin.Flag("upstream.addr", "upstream to connect to").Required().String()
	upstreamPrefixPath = kingpin.Flag("upstream.prefix-path", "upstream prefix path to prepend").String()
	listenAddr         = kingpin.Flag("proxy.listen-addr", "address the proxy will listen on").Required().String()

	urlPattern             = regexp.MustCompile(`^/([^/]+)(/api/v.+)$`)
	supportedQueryPatterns = regexp.MustCompile(`^/api/v1/(query|query_range|query_exemplars)$`)
	supportedMatchPatterns = regexp.MustCompile(`^/api/v1/(series|labels|label/[a-zA-Z0-9_]+/values)$`)
	supportedPlainPatterns = regexp.MustCompile(`^/api/v1/(format_query)$`)
)

const (
	TYPE_QUERY = iota
	TYPE_MATCH
	TYPE_PLAIN
)

func handleQuery(apiType uint, filter string, rw http.ResponseWriter, r *http.Request) {
	log.WithFields(log.Fields{"method": r.Method, "path": r.URL.String(), "filter": filter}).Debug("handling request")
	params := &url.Values{}
	err := r.ParseForm()
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Warn("failed to parse query string")
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	switch apiType {
	case TYPE_QUERY:
		if !r.Form.Has("query") {
			log.WithFields(log.Fields{"method": r.Method, "path": r.URL.String()}).Warn("missing query parameter")
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
	case TYPE_MATCH:
		if !r.Form.Has("match[]") {
			// This is valid, but we need to ensure that we have at least one
			// matcher.
			r.Form.Add("match[]", "{}")
		}
	case TYPE_PLAIN:
		// no special handling needed
	default:
		log.WithFields(log.Fields{"method": r.Method, "path": r.URL.String()}).Warn("unhandled api type")
		rw.WriteHeader(http.StatusInternalServerError)
		return

	}
	for k, vv := range r.Form {
		log.WithFields(log.Fields{"k": k}).Debug("handling form key")
		switch k {
		case "query":
			if len(vv) != 1 {
				log.WithFields(log.Fields{"method": r.Method, "path": r.URL.String()}).Warn("wrong number of query params")
				rw.WriteHeader(http.StatusBadRequest)
				return
			}
			val, err := addQueryFilter(filter, vv[0])
			if err != nil {
				log.WithFields(log.Fields{"val": vv[0], "err": err}).Warn("failed to add filter")
				rw.WriteHeader(http.StatusBadRequest)
				return
			}
			params.Set(k, val)
		case "match[]":
			log.WithFields(log.Fields{"val": vv}).Debug("rewriting match")
			for _, v := range vv {
				val, err := addQueryFilter(filter, v)
				if err != nil {
					log.WithFields(log.Fields{"val": v, "err": err}).Warn("failed to add filter")
					rw.WriteHeader(http.StatusBadRequest)
					return
				}
				params.Add(k, val)
			}
		case "start":
			fallthrough
		case "end":
			fallthrough
		case "step":
			fallthrough
		case "limit":
			fallthrough
		case "timeout":
			fallthrough
		case "time":
			for _, v := range vv {
				params.Add(k, v)
			}
		default:
			log.WithFields(log.Fields{"key": k, "values": vv}).Warn("unknown param")
			continue
		}
	}
	url := &url.URL{
		Scheme:   "http",
		Host:     *upstream,
		Path:     fmt.Sprintf("%s%s", *upstreamPrefixPath, r.URL.Path), //FIXME
		RawQuery: params.Encode(),
	}
	log.WithFields(log.Fields{"url": url.String()}).Debug("starting request to upstream")
	resp, err := http.Get(url.String())
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		log.WithFields(log.Fields{"err": err}).Warn("upstream request failed")
		return
	}
	h := rw.Header()
	for k, vv := range resp.Header {
		if k == "Content-Length" {
			continue
		}
		for _, v := range vv {
			log.WithFields(log.Fields{"header": k, "value": v}).Debug("copying response header")
			h.Add(k, v)
		}
	}
	rw.WriteHeader(resp.StatusCode)
	_, err = io.Copy(rw, resp.Body)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		log.WithFields(log.Fields{"err": err}).Warn("forwarding upstream response failed")
		return
	}
}

func handleUnsupported(rw http.ResponseWriter, r *http.Request) {
	rw.WriteHeader(http.StatusBadRequest)
	rw.Write([]byte("Unsupported\n"))
	log.WithFields(log.Fields{"method": r.Method, "path": r.URL.String()}).Warn("unsupported request")
}

type router struct {
}

func (r router) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" && req.Method != "POST" {
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte("Unsupported method\n"))
		log.WithFields(log.Fields{"method": req.Method, "path": req.URL.String()}).Warn("unsupported method")
		return
	}
	path := req.URL.Path
	m := urlPattern.FindStringSubmatch(path)
	if len(m) != 3 {
		handleUnsupported(rw, req)
		return
	}
	filter := "{" + m[1] + "}"
	apiPath := m[2]
	var apiType uint
	if supportedQueryPatterns.MatchString(apiPath) {
		apiType = TYPE_QUERY
	} else if supportedMatchPatterns.MatchString(apiPath) {
		apiType = TYPE_MATCH
	} else if supportedPlainPatterns.MatchString(apiPath) {
		apiType = TYPE_PLAIN
	} else {
		rw.WriteHeader(http.StatusBadRequest)
		rw.Write([]byte("prometheus-filter-proxy: Unsupported path\n"))
		log.WithFields(log.Fields{"method": req.Method, "path": req.URL.String()}).Warn("unsupported path")
		return
	}
	req.URL.Path = apiPath
	handleQuery(apiType, filter, rw, req)
}

func main() {
	kingpin.Parse()
	if *verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
	log.WithFields(log.Fields{"upstream.url": *upstream, "proxy.listen-addr": *listenAddr}).Info("Starting")
	router := router{}
	http.Handle("/", router)
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
