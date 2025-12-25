package workerpool

type Task func()

type Pool interface {
	Go(work Task)
	Release()
}

type tsk struct {
	Arg  int
	Iter int
}
