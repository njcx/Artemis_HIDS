package main

import (
	"fmt"
)

type nj interface {
	call()
}

type cx interface {
	call()
}

type njcx struct {


}

type tudo struct {

}


func (x *njcx)call()  {

	fmt.Println('1')

}

func (x *tudo) call() {

	fmt.Println('x')

}




func main()  {

	//fmt.Println(info)

	var xx  nj= new(njcx)
	xx.call()

}
