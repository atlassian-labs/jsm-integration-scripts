package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/alexcesaro/log"
	"github.com/alexcesaro/log/golog"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var API_KEY = ""
var TOTAL_TIME = 60
var parameters = map[string]string{}
var configParameters = map[string]string{"apiKey": API_KEY,
	"jsm.api.url":                "https://api.atlassian.com",
	"oem2jsm.logger":             "warning",
	"oem2jsm.http.proxy.enabled": "false",
	"oem2jsm.http.proxy.port":    "1111", "oem2jsm.http.proxy.host": "localhost",
	"oem2jsm.http.proxy.protocol": "http",
	"oem2jsm.http.proxy.username": "",
	"oem2jsm.http.proxy.password": ""}
var configPath = "/etc/jsm/conf/integration.conf"
var levels = map[string]log.Level{"info": log.Info, "debug": log.Debug, "warning": log.Warning, "error": log.Error}
var logger log.Logger

func main() {
	configFile, err := os.Open(configPath)
	if err == nil {
		readConfigFile(configFile)
	} else {
		panic(err)
	}
	version := flag.String("v", "", "")
	parseFlags()

	logger = configureLogger()

	printConfigToLog()

	if *version != "" {
		fmt.Println("Version: 1.1")
		return
	}

	http_post()
}

func printConfigToLog() {
	if logger != nil {
		if logger.LogDebug() {
			logger.Debug("Config:")
			for k, v := range configParameters {
				if strings.Contains(k, "password") {
					logger.Debug(k + "=*******")
				} else {
					logger.Debug(k + "=" + v)
				}
			}
		}
	}
}

func readConfigFile(file io.Reader) {
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") && line != "" {
			l := strings.SplitN(line, "=", 2)
			l[0] = strings.TrimSpace(l[0])
			l[1] = strings.TrimSpace(l[1])
			configParameters[l[0]] = l[1]
			if l[0] == "timeout" {
				TOTAL_TIME, _ = strconv.Atoi(l[1])
			}
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

func configureLogger() log.Logger {
	level := configParameters["oem2jsm.logger"]
	var logFilePath = parameters["logPath"]

	if len(logFilePath) == 0 {
		logFilePath = "/var/log/jsm/oem2jsm.log"
	}

	var tmpLogger log.Logger

	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)

	if err != nil {
		fmt.Println("Could not create log file \""+logFilePath+"\", will log to \"/tmp/oem2jsm.log\" file. Error: ", err)

		fileTmp, errTmp := os.OpenFile("/tmp/oem2jsm.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)

		if errTmp != nil {
			fmt.Println("Logging disabled. Reason: ", errTmp)
		} else {
			tmpLogger = golog.New(fileTmp, levels[strings.ToLower(level)])
		}
	} else {
		tmpLogger = golog.New(file, levels[strings.ToLower(level)])
	}

	return tmpLogger
}

func getHttpClient(timeout int) *http.Client {
	seconds := (TOTAL_TIME / 12) * 2 * timeout
	var proxyEnabled = configParameters["oem2jsm.http.proxy.enabled"]
	var proxyHost = configParameters["oem2jsm.http.proxy.host"]
	var proxyPort = configParameters["oem2jsm.http.proxy.port"]
	var scheme = configParameters["oem2jsm.http.proxy.protocol"]
	var proxyUsername = configParameters["oem2jsm.http.proxy.username"]
	var proxyPassword = configParameters["oem2jsm.http.proxy.password"]
	proxy := http.ProxyFromEnvironment

	if proxyEnabled == "true" {

		u := new(url.URL)
		u.Scheme = scheme
		u.Host = proxyHost + ":" + proxyPort
		if len(proxyUsername) > 0 {
			u.User = url.UserPassword(proxyUsername, proxyPassword)
		}

		if logger != nil {
			logger.Debug("Formed Proxy url: ", u)
		}
		proxy = http.ProxyURL(u)
	}
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           proxy,
			Dial: func(netw, addr string) (net.Conn, error) {
				conn, err := net.DialTimeout(netw, addr, time.Second*time.Duration(seconds))
				if err != nil {
					if logger != nil {
						logger.Error("Error occurred while connecting: ", err)
					}
					return nil, err
				}
				conn.SetDeadline(time.Now().Add(time.Second * time.Duration(seconds)))
				return conn, nil
			},
		},
	}
	return client
}

func http_post() {
	parameters["apiKey"] = configParameters["apiKey"]

	var logPrefix = "[OEM2JiraServiceManagement] "

	apiUrl := configParameters["jsm.api.url"] + "/jsm/ops/integration/v1/json/oem"
	viaMaridUrl := configParameters["viaMaridUrl"]
	target := ""

	if viaMaridUrl != "" {
		apiUrl = viaMaridUrl
		target = "Marid"
	} else {
		target = "JSM"
	}

	if logger != nil {
		logger.Debug("URL: ", apiUrl)
		logger.Debug("Data to be posted:")
		logger.Debug(parameters)
	}

	var tmpLogPath string

	if val, ok := parameters["logPath"]; ok {
		tmpLogPath = val
		delete(parameters, "logPath")
	}

	var buf, _ = json.Marshal(parameters)

	parameters["logPath"] = tmpLogPath

	body := bytes.NewBuffer(buf)
	request, _ := http.NewRequest("POST", apiUrl, body)
	for i := 1; i <= 3; i++ {
		client := getHttpClient(i)

		if logger != nil {
			logger.Debug(logPrefix+"Trying to send data to "+target+" with timeout: ", (TOTAL_TIME/12)*2*i)
		}

		resp, error := client.Do(request)

		if error == nil {
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)

			if err == nil {
				if resp.StatusCode == 200 {
					if logger != nil {
						logger.Debug(logPrefix + " Response code: " + strconv.Itoa(resp.StatusCode))
						logger.Debug(logPrefix + "Response: " + string(body[:]))
						logger.Info(logPrefix + "Data from OEM posted to " + target + " successfully")
					}
				} else {
					if logger != nil {
						logger.Error(logPrefix + "Couldn't post data from OEM to " + target + " successfully; Response code: " + strconv.Itoa(resp.StatusCode) + " Response Body: " + string(body[:]))
					}
				}
			} else {
				if logger != nil {
					logger.Error(logPrefix+"Couldn't read the response from "+target, err)
				}
			}
			break
		} else if i < 3 {
			if logger != nil {
				logger.Error(logPrefix+"Error occurred while sending data, will retry.", error)
			}
		} else {
			if logger != nil {
				logger.Error(logPrefix+"Failed to post data from OEM.", error)
			}
		}
		if resp != nil {
			defer resp.Body.Close()
		}
	}
}

func parseFlags() map[string]string {
	apiKey := flag.String("apiKey", "", "apiKey")
	tags := flag.String("tags", "", "tags")
	responders := flag.String("responders", "", "responders")
	logPath := flag.String("logPath", "", "LOGPATH")

	flag.Parse()

	args := flag.Args()
	for i := 0; i < len(args); i += 2 {
		if len(args)%2 != 0 && i == len(args)-1 {
			parameters[args[i]] = ""
		} else {
			parameters[args[i]] = args[i+1]
		}
	}

	if *apiKey != "" {
		configParameters["apiKey"] = *apiKey
	}

	if *tags != "" {
		parameters["tags"] = *tags
	} else {
		parameters["tags"] = configParameters["tags"]
	}

	if *responders != "" {
		parameters["responders"] = *responders
	} else {
		parameters["responders"] = configParameters["responders"]
	}

	if *logPath != "" {
		parameters["logPath"] = *logPath
	} else {
		parameters["logPath"] = configParameters["logPath"]
	}

	var envVars map[string]string = make(map[string]string)

	for _, envVar := range os.Environ() {
		pair := strings.SplitN(envVar, "=", 2)

		if pair[0] == "" || pair[1] == "" {
			continue
		}

		envVars[pair[0]] = pair[1]
	}

	for key, value := range envVars {
		parameters[key] = value
	}

	return parameters
}
