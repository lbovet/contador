package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
)

func get() int64 {
	result, err := redisdb.Get("counter1").Result()
	//time.Sleep(5 * time.Second)
	if err != nil {
		panic(err)
	}
	r64, _ := strconv.ParseInt(result, 10, 64)
	return r64
	// return result

}

func incr() int64 {
	result, err := redisdb.IncrBy("counter1", 1).Result()
	if err != nil {
		panic(err)
	}
	return result
}

func count(w http.ResponseWriter, r *http.Request) {
	result := incr()
	cs := strconv.FormatInt(result, 10)

	m := fmt.Sprintf("%s.gontador.%s.%s.counter.value",
		os.Getenv("SYSTEM_INSTANCE"),
		os.Getenv("SYSTEM_ENV"),
		os.Getenv("SERVICE_INSTANCE"),
	)
	graphiteClient.Send(m, string(cs))
	ti := r.Header.Get("X-Trace-Id")
	fmt.Fprint(w, result)

	dlog("Counting: "+cs, string(ti))
}
