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
	outFile       string
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
		Use:   "crayflow-load—var",
		Short: "Load the workflow variable",
		Long:  "Load the workflow variable",
		Run: func(cmd *cobra.Command, args []string) {
			// log.Println("args:", args, ", fromFile:", fromFile)

			if len(os.Args) < 2 {
				panic("too few arguments, expecting key of a variable. usage: " +
					"\n\t./load_var ${key} \n\t./load_var ${key} --file ${value_save_file}")
			}
			key := os.Args[1]

			loadVar(key, outFile)
		},
	}

	cmd.Flags().StringVarP(&outFile, "file", "f", "", "Write value to the file")
	cmd.Execute()
}

func loadVar(key, outFile string) {
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

	value := cm.Data[key]
	if outFile != "" {
		if err := ioutil.WriteFile(outFile, []byte(value), 0755); err != nil {
			panic(err.Error())
		}
		return
	}

	fmt.Println(value)
}
