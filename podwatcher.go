package kubescaler

import (
	"k8s.io/apimachinery/pkg/watch"
	"sync"
)

type PodWatcher struct {
	w      watch.Interface
	wg     sync.WaitGroup
	Events chan watch.Event
}

func (pw *PodWatcher) Watch() {
	go func() {
		pw.wg.Add(1)
		for e := range pw.w.ResultChan() {
			pw.Events <- e
		}
		pw.wg.Done()
	}()
}

func (pw *PodWatcher) Stop() {
	pw.w.Stop()
	pw.wg.Wait()
	if pw.Events != nil {
		close(pw.Events)
	}
}
