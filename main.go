package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"time"
)

var gauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "temperature_fahrenheit",
	Help: "Temperature in fahrenheit",
}, []string{"door"})

func main() {
	hostname := flag.String("hostname", "192.168.1.242", "Hostname of the iSmartGate device")
	username := flag.String("username", "admin", "Username for the iSmartGate device")
	password := flag.String("password", "", "Password for the iSmartGate device")
	interval := flag.Int("interval", 60, "Interval between re-reading the temperature from the iSmartGate device")
	listenPort := flag.Int("listenPort", 9805, "Which port should the prometheus endpoint listen on")

	// TODO: Proper multi-door support
	door := flag.Int("door", 1, "Which door to collect data for")
	flag.Parse()

	if *password == "" {
		log.Printf("You must supply a password")
		return
	}

	updateTemperature(*hostname, *username, *password, *door)

	ticker := time.NewTicker(time.Duration(*interval) * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				updateTemperature(*hostname, *username, *password, *door)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	// Listen and export metrics ...
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(fmt.Sprintf(":%d", *listenPort), nil)
}

func updateTemperature(hostname string, username string, password string, door int) {
	log.Println("Requesting temperature update")
	tmp, err := getTemperature(hostname, username, password, door)
	if err == nil {
		log.Printf("Successfully read temperature %2.2f\n", tmp)
		gauge.WithLabelValues(fmt.Sprintf("door%d", door)).Set(tmp)
	} else {
		log.Printf("Failed temperature update: %v\n", err)
	}
}

// TODO: Reduce auth frequency
func getTemperature(hostname string, username string, password string, door int) (float64, error) {
	cookieJar, _ := cookiejar.New(nil)

	client := &http.Client{
		Jar: cookieJar,
	}

	authURL := fmt.Sprintf("http://%s/index.php", hostname)
	temperatureURL := fmt.Sprintf("http://%s/isg/temperature.php?door=%d", hostname, door)

	resp, err := client.PostForm(authURL,
		url.Values{"send-login": {"Sign In"},
			"login": {username},
			"pass":  {password},
		})

	// TODO: Handle auth failure; it seems like you get a 200 OK no matter what
	if err == nil {
		if resp.StatusCode == 200 {
			defer resp.Body.Close()
			resp, err := client.Get(temperatureURL)
			if err == nil {

				defer resp.Body.Close()
				content, err := ioutil.ReadAll(resp.Body)
				if err == nil {

					var data []string
					json.Unmarshal(content, &data)
					if len(data) == 2 {

						v, err := strconv.ParseInt(data[0], 10, 32)
						if err == nil {
							var tempInF = fToC(float64(v) / 1000)
							log.Printf("Read temperature %2.2fF", tempInF)
							return tempInF, nil
						} else {
							log.Printf("Unable to parse integer from %s", data[0])
							return 0.0, err
						}
					} else {
						log.Printf("Unable to read two value array from response. Content was: %v", string(content))
						return 0.0, err
					}
				} else {
					log.Printf("Error when reading response body was: %v", err)
					return 0.0, err
				}
			} else {
				log.Printf("Error when making request was: %v", err)
				return 0.0, err
			}
		} else {
			log.Printf("Got invalid status code")
			return 0.0, err
		}
	} else {
		log.Printf("Error when trying to authenticate: '%v'\n", err)
		return 0.0, err
	}
	log.Printf("Uhh")
	return 0.0, err
}

// fToc converts celcius to fahrenheit
func fToC(celcius float64) float64 {
	return (celcius * 9 / 5) + float64(32)
}
