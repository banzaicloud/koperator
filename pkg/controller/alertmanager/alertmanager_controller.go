package alertmanager

import (
	"net"
	"net/http"

	"github.com/banzaicloud/kafka-operator/internal/alertmanager"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	receiverAddr = ":9001"
)

type AController struct {
	Client client.Client
}

// Add creates a new Alertmanager Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {

	return mgr.Add(AController{Client: mgr.GetClient()})
}

func (c AController) Start(<-chan struct{}) error {
	logf.SetLogger(logf.ZapLogger(false))
	log := logf.Log.WithName("alertmanager-entrypoint")

	ln, _ := net.Listen("tcp", receiverAddr)
	httpServer := &http.Server{Handler: alertmanager.NewApp(log, c.Client)}
	return httpServer.Serve(ln)
}
