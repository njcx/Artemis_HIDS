package main

import (
	"github.com/njcx/peppa_hids/collect"
	"fmt"
)

func main()  {

	info := collect.GetAllInfo()

	fmt.Println(info)

}