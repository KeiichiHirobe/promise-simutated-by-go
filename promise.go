package main

import (
	"fmt"
	"sync"
	"time"
)

// https://ja.javascript.info/async
// https://promisesaplus.com/

// もし返却された値が promise である場合、それ以降の実行はその promise が解決するまで中断されます
// 技術的には、任意の型の引数で reject を呼び出すことが可能です(resolve のように)。しかし、reject (またはそれを継承したもの)では、Error オブジェクトを利用することを推奨します。その理由は後ほど明らかになります。
// promise が pending の場合、.then/catch ハンドラは結果を待ちます。そうではなく、promise がすでに settled である場合は直ちに実行されます。:
// promise が reject されると、コントロールはチェーンに沿って最も近い reject ハンドラにジャンプします
// 単一の Promise に複数の .then を追加することもできます
// 正確には、.then は任意の “thenable” オブジェクトを返す可能性があり、それは promise として同じように扱われます。
//“thenable” オブジェクトとは、メソッド .then を持つオブジェクトです。
// promise の動作ルールにより、.then/catch ハンドラが値(エラーオブジェクトか他のなにかなのかは関係ありません)を返した場合、実行は “正常の” フローを続けます。
//   * JSでは、then内でthrowすることでrejectすることができる

// Promise.all

type State int

const (
	PENDING   State = iota + 1 // initial state, neither fulfilled nor rejected.
	FULFILLED                  // meaning that the operation was completed successfully.
	REJECT                     // meaning that the operation failed.
)

type Thenable interface {
	Then(resolve func(interface{}), reject func(interface{}))
}

type Promise struct {
	mu    sync.Mutex
	done  chan struct{}
	state State
	// In most cases, result is error when rejected.However, it is not always necessary.
	result interface{}
}

var closedChan = func() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()

/*
promiseオブジェクトを受け取った場合
受け取ったpromiseオブジェクトをそのまま返す

thenableなオブジェクトを受け取った場合
then をもつオブジェクトを新たなpromiseオブジェクトにして返す

その他の値(オブジェクトやnull等も含む)を受け取った場合
その値でresolveされる新たなpromiseオブジェクトを作り返す
*/

func Resolved(result interface{}) *Promise {
	if promise, ok := result.(*Promise); ok {
		return promise
	} else if thenable, ok := result.(Thenable); ok {
		return NewPromise(
			func(resolve func(interface{}), reject func(interface{})) {
				thenable.Then(resolve, reject)
			},
		)
	} else {
		return &Promise{
			done:   closedChan,
			state:  FULFILLED,
			result: result,
		}
	}
}

/*
受け取った値でrejectされた新たなpromiseオブジェクトを返す。
Promise.rejectに渡す値は Error オブジェクトとすべきである。
また、Promise.resolveとは異なり、promiseオブジェクトを渡した場合も常に新たなpromiseオブジェクトを作成する。
*/

func Rejected(result interface{}) *Promise {
	return &Promise{
		done:   closedChan,
		state:  REJECT,
		result: result,
	}
}

func NewPromise(task func(resolve func(interface{}), reject func(interface{}))) *Promise {
	p := &Promise{
		state: PENDING,
		done:  make(chan struct{}),
	}
	task(
		func(result interface{}) {
			p.resolveWith(result)
		},
		func(result interface{}) {
			p.rejectWith(result)
		},
	)
	return p
}

func (p *Promise) resolveWith(result interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state != PENDING {
		return
	}
	p.result = result
	p.state = FULFILLED
	close(p.done)
}

func (p *Promise) rejectWith(result interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state != PENDING {
		return
	}
	p.result = result
	p.state = REJECT
	close(p.done)
}

// Then calls resolve/reject when promise has been settled.
// resolve/reject may be nil. If nil, do nothing and throw to next Then.
func (p *Promise) Then(onFulfilled func(interface{}) interface{}, onRejected func(interface{}) interface{}) *Promise {
	rp := &Promise{
		state: PENDING,
		done:  make(chan struct{}),
	}
	// settled でも必ず非同期に実行する
	go func() {
		<-p.done
		var result interface{}
		// no need to lock here, because promise p is read only after done
		switch p.state {
		case PENDING:
			panic("bug")
		case FULFILLED:
			if onFulfilled == nil {
				result = p.result
			} else {
				result = onFulfilled(p.result)
			}
		case REJECT:
			if onRejected == nil {
				result = p.result
			} else {
				result = onRejected(p.result)
			}
		}
		resultPromise := Resolved(result)
		<-resultPromise.done

		// Then内でreturnされたpromiseは、promise自身でなく、その解決された値が渡される
		if promise, ok := result.(*Promise); ok {
			result = promise.result
		}
		switch resultPromise.state {
		case PENDING:
			panic("bug")
		case FULFILLED:
			rp.resolveWith(result)
		case REJECT:
			rp.rejectWith(result)
		}
	}()
	return rp
}

func (p *Promise) Finally(onFinally func()) *Promise {
	return p.Then(
		func(result interface{}) interface{} {
			onFinally()
			return result
		},
		func(result interface{}) interface{} {
			onFinally()
			return Rejected(result)
		},
	)
}

func main() {
	var done = make(chan struct{})
	NewPromise(
		func(resolve func(interface{}), reject func(interface{})) {
			time.Sleep(3 * time.Second)
			resolve(1)
			fmt.Println("1================================")
		},
	).Then(
		func(i interface{}) interface{} {
			fmt.Println("2================================")
			fmt.Println(i)
			return NewPromise(
				func(resolve func(interface{}), reject func(interface{})) {
					time.Sleep(1 * time.Second)
					resolve(i.(int) * 2)
				},
			)
		},
		nil,
	).Then(
		func(i interface{}) interface{} {
			fmt.Println("3================================")
			fmt.Println(i)
			return NewPromise(
				func(resolve func(interface{}), reject func(interface{})) {
					time.Sleep(1 * time.Second)
					resolve(i.(int) * 2)
				},
			)
		},
		nil,
	).Then(
		func(i interface{}) interface{} {
			fmt.Println(i)
			return nil
		},
		nil,
	).Then(
		func(i interface{}) interface{} {
			fmt.Println("done")
			close(done)
			return nil
		},
		nil,
	)
	fmt.Println("=====================")
	<-done
}
