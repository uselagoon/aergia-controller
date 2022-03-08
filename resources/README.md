## Selectors

The idler uses label selectors at different stages through the idling process to determine what it should and shouldn't idle.

The first selector is namespace labels, this will select only namespaces that have the defined label selectors.

Second is build labels, this is for lagoon only, and will check the namespace for any running lagoon builds with matching labels.

Third is deployments, this checks the namespace for deployments to get the number of running replicas for each deployment to see if it is already idled or not

Fourth is pods, this is where it checks the running uptime of a pod and will actually perform the idling

## Operators

When using label selectors, the operators to use in the selectors yaml are in the hand right column

```
type Operator string
const (
	DoesNotExist Operator = "!"
	Equals       Operator = "="
	DoubleEquals Operator = "=="
	In           Operator = "in"
	NotEquals    Operator = "!="
	NotIn        Operator = "notin"
	Exists       Operator = "exists"
	GreaterThan  Operator = "gt"
	LessThan     Operator = "lt"
)
```