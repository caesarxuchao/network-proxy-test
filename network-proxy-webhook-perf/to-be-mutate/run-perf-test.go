package main

import (
	"flag"
	"fmt"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func createConfigmap(c *kubernetes.Clientset, index int) error {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("configmap-%d", index),
		},
		Data: map[string]string{
			"mutation-start": "yes",
		},
	}
	_, err := c.CoreV1().ConfigMaps("default").Create(cm)
	return err
}

func batchCreate(c *kubernetes.Clientset, batchIndex, batchSize int) error {
	start := batchIndex * batchSize
	for i := start; i < start+batchSize; i++ {
		err := createConfigmap(c, i)
		if err != nil {
			return err
		}
	}
	return nil
}

var (
	kubeconfigPath string
	batchSize      int
	batchNumber    int
)

func main() {
	flag.StringVar(&kubeconfigPath, "kp", "/home/xuchao/kubeconfig", "")
	flag.IntVar(&batchSize, "bs", 10, "")
	flag.IntVar(&batchNumber, "bn", 10, "")
	flag.Parse()
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	errCh := make(chan (error), batchNumber)
	var wg sync.WaitGroup
	wg.Add(batchNumber)
	start := time.Now()
	for i := 0; i < batchNumber; i++ {
		ii := i
		go func() {
			defer wg.Done()
			err := batchCreate(c, ii, batchSize)
			errCh <- err
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)
	fmt.Printf("CHAO: test run time: %v\n", elapsed)
	close(errCh)
	for err := range errCh {
		if err != nil {
			fmt.Printf("%v\n", err)
		}
	}
}
