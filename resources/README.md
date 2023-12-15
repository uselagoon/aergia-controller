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

## Blocked User Agents and IP Allow Lists

This supports globally 
* blocking user agents via a `blockedagents` file which is a single line per entry of useragents or regex patterns to match against. These must be `go` based regular expressions.
* blocking IP addresses via `blockips` file which is a single line per entry of ip address to block
* allowing IP addresses via `allowips` file which is a single line per entry of ip address to allow

There are also annotations that can be added to specific ingress objects that allow for ip allowlist or specific user agent blocking.
* `idling.amazee.io/ip-allow-list` - a comma separated list of ip addresses to allow, will be checked against x-forward-for, but if true-client-ip is provided it will prefer this.
* `idling.amazee.io/ip-block-list` - a comma separated list of ip addresses to allow, will be checked against x-forward-for, but if true-client-ip is provided it will prefer this.
* `idling.amazee.io/blocked-agents` - a comma separated list of user agents or regex patterns to block.

> Note: Providing the annotations overrides the global block list, it does not append.

##### blockedagents file example
```
@(example|internal).test.?$
```