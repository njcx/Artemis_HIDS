package main

import (
	"peppa_hids/collect"
	"fmt"
)

func main()  {

	info := collect.GetAllInfo()

	fmt.Println(info)

}