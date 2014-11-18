{
    "id": "AutoScaler",
    "kind": "Pod",
    "apiVersion": "v1beta1",
    "desiredState": {
	"manifest": {
	    "version": "v1beta1",
	    "id": "AutoScaler",
	    "containers": [{
		"name": "scaler",
		"image": "vish/scaler:test",
	    }]
	}
    },
    "labels": {
	"service": "autoscaler",
    }
}