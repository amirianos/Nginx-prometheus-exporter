package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Access_log_path string `yaml:"access_log_path"`
	Port            string `yaml:"port"`
	Log_format      string `yaml:"log_format"`
}

var config Config
var requests = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "requests",
	},
	[]string{
		"path",
	},
)

func nginx_parser(path string, log_format string) {
	//path := config.access_log_path
	//log_format := config.log_format
	requests.Reset()
	f, err := os.Open(path)

	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)
	logsFormat := log_format
	regexFormat := regexp.MustCompile(`\$([\w_]*)`).ReplaceAllString(logsFormat, `(?P<$1>.*)`)

	re := regexp.MustCompile(regexFormat)
	for scanner.Scan() {
		logsExample := scanner.Text()
		matches := re.FindStringSubmatch(logsExample)
		nginx_date_time := matches[15]
		nginx_date := strings.Split(nginx_date_time, "T")[0]
		nginx_time := strings.Split(nginx_date_time, "T")[1]
		layout := "2006-01-02 15:04:05"
		nginx_date_time = nginx_date + " " + nginx_time
		log_time, err := time.Parse(layout, nginx_date_time)
		if err != nil {
			log.Fatal(err)
		}
		now_time := time.Now()
		sub_time := now_time.Add(-time.Minute * 10)
		sub_time_string := sub_time.Format(layout)
		sub_time, _ = time.Parse(layout, sub_time_string)
		http_host := matches[8]
		if sub_time.Before(log_time) {
			requests.WithLabelValues(http_host).Add(1)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

}
func main() {
	data, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Fatal(err)
		return
	}

	errr := yaml.Unmarshal(data, &config)
	fmt.Println(errr)
	reg := prometheus.NewRegistry()
	reg.Register(requests)
	router := mux.NewRouter()
	router.Path("/prometheus").Handler(promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	ticker := time.NewTicker(10 * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				nginx_parser(config.Access_log_path, config.Log_format)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
	err = http.ListenAndServe(":"+config.Port, router)
	log.Fatal(err)
}
