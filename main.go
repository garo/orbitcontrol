package main

import . "github.com/garo/containrunner/containrunner"
import "fmt"

func main() {

	var containrunner Containrunner
	containrunner.Tags = []string{"testtag"}
	containrunner.EtcdEndpoints = []string{"http://etcd:4001"}

	fmt.Printf("Starting containrunner with config %+v\n", containrunner)
	containrunner.Start()
	containrunner.Wait()

}
