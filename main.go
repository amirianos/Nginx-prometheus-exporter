package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var requests = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "requests",
	},
	[]string{
		"path",
	},
)

func nginx_parser() {
	requests.Reset()
	f, err := os.Open("/var/log/nginx/access.log")

	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)
	logsFormat := `$ip_address - \[$time_stamp\] \"$http_method $request_path $_\" $response_code $response_size $http_host $http_user_agent \($http_x_forwarded_for\) $_ $browser - $request_time_spent $upstream_response_time $request_date`
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
				nginx_parser()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
	err := http.ListenAndServe(":5000", router)
	log.Fatal(err)
}
