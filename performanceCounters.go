package main

import (
        "context"
        "encoding/csv"
        "flag"
        "fmt"
        "log"
        "net/url"
        "os"
        "strings"

        "github.com/vmware/govmomi"
        "github.com/vmware/govmomi/vim25/mo"
)

// GetEnvString returns string from environment variable.
func GetEnvString(v string, def string) string {
        r := os.Getenv(v)
        if r == "" {
                return def
        }

        return r
}

// GetEnvBool returns boolean from environment variable.
func GetEnvBool(v string, def bool) bool {
        r := os.Getenv(v)
        if r == "" {
                return def
        }

        switch strings.ToLower(r[0:1]) {
        case "t", "y", "1":
                return true
        }

        return false
}

const (
        envURL      = "GOVMOMI_URL"
        envUserName = "GOVMOMI_USERNAME"
        envPassword = "GOVMOMI_PASSWORD"
        envInsecure = "GOVMOMI_INSECURE"
)

var urlDescription = fmt.Sprintf("ESX or vCenter URL [%s]", envURL)
var urlFlag = flag.String("url", GetEnvString(envURL, "https://username:password@host/sdk"), urlDescription)

var insecureDescription = fmt.Sprintf("Don't verify the server's certificate chain [%s]", envInsecure)
var insecureFlag = flag.Bool("insecure", GetEnvBool(envInsecure, false), insecureDescription)

func processOverride(u *url.URL) {
        envUsername := os.Getenv(envUserName)
        envPassword := os.Getenv(envPassword)

        // Override username if provided
        if envUsername != "" {
                var password string
                var ok bool

                if u.User != nil {
                        password, ok = u.User.Password()
                }

                if ok {
                        u.User = url.UserPassword(envUsername, password)
                } else {
                        u.User = url.User(envUsername)
                }
        }

        // Override password if provided
        if envPassword != "" {
                var username string

                if u.User != nil {
                        username = u.User.Username()
                }

                u.User = url.UserPassword(username, envPassword)
                        }
}

func exit(err error) {
        fmt.Fprintf(os.Stderr, "Error: %s\n", err)
        os.Exit(1)
}

func main() {
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()

        flag.Parse()

        // Parse URL from string
        u, err := url.Parse(*urlFlag)
        if err != nil {
                exit(err)
        }

        fmt.Println("u:", u)

        // Override username and/or password as required
        processOverride(u)

        // Connect and log in to ESX or vCenter
        client, err := govmomi.NewClient(ctx, u, *insecureFlag)
        if err != nil {
                exit(err)
        }

        if client.IsVC() {
                fmt.Println("connected to vCenter")
        } else {
                fmt.Println("connected to ESXi host")
        }

        var perfManager mo.PerformanceManager
        err = client.RetrieveOne(ctx, *client.ServiceContent.PerfManager, nil, &perfManager)
        perfCounters := perfManager.PerfCounter

        counterDetails := make([][]string, 0)

        for _, perfCounter := range perfCounters {
                groupInfo := perfCounter.GroupInfo.GetElementDescription()
                nameInfo := perfCounter.NameInfo.GetElementDescription()

                fullName := groupInfo.Key + "." + nameInfo.Key + "." + fmt.Sprint(perfCounter.RollupType)
                counterDetails = append(counterDetails, []string{fullName, fmt.Sprint(perfCounter.Level), nameInfo.Summary})
        }

        outputFile, err := os.Create("performanceCounters.csv")
        csvWriter := csv.NewWriter(outputFile)
        csvWriter.WriteAll(counterDetails)

        if err := csvWriter.Error(); err != nil {
                log.Fatalln("error writing csv:", err)
        }
}
