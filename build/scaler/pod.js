{
    "id": "kuberscaler",
    "kind": "Pod",
    "apiVersion": "v1beta1",
    "desiredState": {
	"manifest": {
	    "version": "v1beta1",
	    "id": "kuberscaler",
	    "containers": [{
		"name": "provisioner",
		"image": "vish/provisioner:test",
	    }, {
		"name": "statscollector",
		"image": "vish/statscollector:test",
	    }, {
		"name": "scaler",
		"image": "vish/scaler:test",
	    }]
	}
    },
    "labels": {
	"service": "kuberscaler",
    }
}
