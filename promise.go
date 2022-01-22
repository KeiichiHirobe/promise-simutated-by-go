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
	Then(resolve func(interface{}), reject func(interface{})) *Promise
}

// RejectedError represents error which is returned by Then, and result will be recognized as rejected.
type RejectedError interface {
	IsRejected() bool
}

func isRejectedResult(result interface{}) bool {
	if err, ok := result.(RejectedError); ok && err.IsRejected() {
		return true
	}
	return false
}

type Promise struct {
	mu    sync.Mutex
	done  chan struct{}
	state State
	// In most cases, result is error when rejected.However, it is not always necessary.
	result interface{}
}

func ResolvedPromise(result interface{}) *Promise {
	return &Promise{
		state:  FULFILLED,
		result: result,
	}
}

func RejectedPromise(result interface{}) *Promise {
	return &Promise{
		state:  REJECT,
		result: result,
	}
}

func promiseFrom(result interface{}) *Promise {
	if promise, ok := result.(*Promise); ok {
		return promise
	}
	/*
		if thenable, ok := result.(Thenable); ok {
			return thenable.Then()
		}
	*/
	if isRejectedResult(result) {
		return RejectedPromise(result)
	} else {
		return ResolvedPromise(result)
	}
}

func NewPromise(task func(resolve func(interface{}), reject func(interface{}))) *Promise {

	p := &Promise{
		state: PENDING,
		done:  make(chan struct{}),
	}
	// ここでgo が必要かは確認
	go task(
		func(result interface{}) {
			p.ResolveWith(result)
		},
		func(result interface{}) {
			p.RejectWith(result)
		},
	)
	return p
}

func (p *Promise) ResolveWith(result interface{}) {
	p.mu.Lock()
	p.result = result
	p.state = FULFILLED
	p.mu.Unlock()
	close(p.done)
}

func (p *Promise) RejectWith(result interface{}) {
	p.mu.Lock()
	p.result = result
	p.state = REJECT
	p.mu.Unlock()
	close(p.done)
}

func (p *Promise) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// Then calls resolve/reject when promise has been settled.
// resolve/reject may be nil. If nil, do nothing and throw to next Then.
func (p *Promise) Then(resolve func(interface{}) interface{}, reject func(interface{}) interface{}) *Promise {
	p.mu.Lock()
	state := p.state
	p.mu.Unlock()
	switch state {
	case PENDING:
	case FULFILLED:
		if resolve == nil {
			return ResolvedPromise(p.result)
		} else {
			return promiseFrom(resolve(p.result))
		}
	case REJECT:
		if reject == nil {
			return RejectedPromise(p.result)
		} else {
			return promiseFrom(reject(p.result))
		}
	}

	rp := &Promise{
		state: PENDING,
		done:  make(chan struct{}),
	}

	go func() {
		<-p.done
		var result interface{}
		switch p.state {
		case PENDING:
			panic("bug")
		case FULFILLED:
			if resolve == nil {
				result = p.result
			} else {
				result = resolve(p.result)
			}
		case REJECT:
			if reject == nil {
				result = p.result
			} else {
				result = reject(p.result)
			}
		}

		if promise, ok := result.(*Promise); ok {
			_ = promise.Then(
				func(result interface{}) interface{} {
					rp.ResolveWith(result)
					return nil
				},
				func(result interface{}) interface{} {
					rp.RejectWith(result)
					return nil
				},
			)
			return
		}

		/*
			if thenable, ok := value.(Thenable); ok {
				return &Promise{}
			}
		*/

		if isRejectedResult(result) {
			rp.RejectWith(result)
		} else {
			rp.ResolveWith(result)
		}
	}()

	return rp
}

/*
new Promise(function(resolve, reject) {

  setTimeout(() => resolve(1), 1000);

}).then(function(result) {

  alert(result); // 1

  return new Promise((resolve, reject) => { // (*)
    setTimeout(() => resolve(result * 2), 1000);
  });

}).then(function(result) { // (**)

  alert(result); // 2

  return new Promise((resolve, reject) => {
    setTimeout(() => resolve(result * 2), 1000);
  });

}).then(function(result) {

  alert(result); // 4

});

*/

func main() {
	var done = make(chan struct{})
	NewPromise(
		func(resolve func(interface{}), reject func(interface{})) {
			time.Sleep(1 * time.Second)
			resolve(1)
		},
	).Then(
		func(i interface{}) interface{} {
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

	<-done
}
