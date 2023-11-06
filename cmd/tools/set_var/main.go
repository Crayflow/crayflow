package main

import (
	"context"
	"fmt"
	devopsv1 "github.com/buhuipao/crayflow/api/v1"
	"github.com/spf13/cobra"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
)

var (
	configMapName string
	namespace     string
	fromFile      string
)

func init() {
	name := os.Getenv(devopsv1.EnvCrayflowWorkflowNameKey)
	if name == "" {
		panic("not set workflow name environment variable")
	}

	namespace = os.Getenv(devopsv1.EnvCrayflowWorkflowNamespaceKey)
	if namespace == "" {
		panic("not set workflow namespace environment variable")
	}

	configMapName = fmt.Sprintf(devopsv1.WorkflowVariableKeyFormat, name)
}

func main() {
	cmd := &cobra.Command{
		Use:   "crayflow-set—var",
		Short: "Set the workflow variable",
		Long:  "Set the workflow variable, and can be load after this node",
		Run: func(cmd *cobra.Command, args []string) {
			// log.Println("args:", args, ", fromFile:", fromFile)

			if len(args) < 2 {
				panic("too few arguments, expecting key and value of a variable. usage: " +
					"\n\t./set_var ${key} ${value} \n\t./set_var ${key} --file ${value_file_path}")
			}
			key := args[1]

			var value string
			if len(args) >= 3 {
				value = os.Args[2]
			}

			setVar(key, value, fromFile)
		},
	}

	cmd.Flags().StringVarP(&fromFile, "file", "f", "", "Read value from the file")
	cmd.Execute()
}

func setVar(key, value, fromFile string) {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configMapName, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	// log.Printf("Get configMap[%s/%s], data: %v\n", cm.Namespace, cm.Name, cm.Data)
	if value == "" && fromFile != "" {
		bs, err := ioutil.ReadFile(fromFile)
		if err != nil {
			panic(err.Error())
		}
		value = string(bs)
	}

	// log.Printf("update configMap[%s/%s], key: %s, value: %s\n", cm.Namespace, cm.Name, key, value)
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[key] = value
	_, err = clientset.CoreV1().ConfigMaps(namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
	if err != nil {
		panic(err.Error())
	}
}
