package main

import (
    "fmt"
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "infraX control plane (skeleton)")
    })
    log.Println("control plane listening :9090")
    log.Fatal(http.ListenAndServe(":9090", nil))
}
