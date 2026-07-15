# API Reference

Packages:

- [edp.epam.com/v1alpha1](#edpepamcomv1alpha1)

# edp.epam.com/v1alpha1

Resource Types:

- [PipelineRunQueue](#pipelinerunqueue)




## PipelineRunQueue
<sup><sup>[↩ Parent](#edpepamcomv1alpha1 )</sup></sup>






PipelineRunQueue is the Schema for the pipelinerunqueues API

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>edp.epam.com/v1alpha1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>PipelineRunQueue</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#pipelinerunqueuespec">spec</a></b></td>
        <td>object</td>
        <td>
          spec defines the desired state of PipelineRunQueue<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#pipelinerunqueuestatus">status</a></b></td>
        <td>object</td>
        <td>
          status defines the observed state of PipelineRunQueue<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PipelineRunQueue.spec
<sup><sup>[↩ Parent](#pipelinerunqueue)</sup></sup>



spec defines the desired state of PipelineRunQueue

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#pipelinerunqueuespecselector">selector</a></b></td>
        <td>object</td>
        <td>
          Selector picks the PipelineRuns in this namespace governed by the queue. Required.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>concurrency</b></td>
        <td>integer</td>
        <td>
          Concurrency is the max number of concurrently running PipelineRuns per lane.<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Default</i>: 1<br/>
            <i>Minimum</i>: 1<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>queueKey</b></td>
        <td>[]string</td>
        <td>
          QueueKey lists PipelineRun label keys whose values form the lane identity.
Runs whose values differ for any key land in independent lanes.
Empty means the queue forms a single lane.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>strategy</b></td>
        <td>enum</td>
        <td>
          Strategy determines how the queue treats already-admitted runs when a newer
PipelineRun arrives in the same lane.<br/>
          <br/>
            <i>Enum</i>: Queue, ReplaceQueued, CancelInProgress<br/>
            <i>Default</i>: Queue<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PipelineRunQueue.spec.selector
<sup><sup>[↩ Parent](#pipelinerunqueuespec)</sup></sup>



Selector picks the PipelineRuns in this namespace governed by the queue. Required.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#pipelinerunqueuespecselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions is a list of label selector requirements. The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels is a map of {key,value} pairs. A single {key,value} in the matchLabels
map is equivalent to an element of matchExpressions, whose key field is "key", the
operator is "In", and the values array contains only "value". The requirements are ANDed.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PipelineRunQueue.spec.selector.matchExpressions[index]
<sup><sup>[↩ Parent](#pipelinerunqueuespecselector)</sup></sup>



A label selector requirement is a selector that contains values, a key, and an operator that
relates the key and values.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values.
Valid operators are In, NotIn, Exists and DoesNotExist.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string values. If the operator is In or NotIn,
the values array must be non-empty. If the operator is Exists or DoesNotExist,
the values array must be empty. This array is replaced during a strategic
merge patch.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PipelineRunQueue.status
<sup><sup>[↩ Parent](#pipelinerunqueue)</sup></sup>



status defines the observed state of PipelineRunQueue

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#pipelinerunqueuestatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          Conditions represent the current state of the PipelineRunQueue resource.
Each condition has a unique type and reflects the status of a specific aspect of the resource.

Standard condition types include:
- "Ready": the queue is reconciled and its lane projection is up to date
- "Degraded": the controller failed to reach or maintain the desired state

The status of each condition is one of True, False, or Unknown.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#pipelinerunqueuestatuslanesindex">lanes</a></b></td>
        <td>[]object</td>
        <td>
          Lanes is a projection of the derived queue state for observability;
the source of truth is always the live PipelineRun set.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          ObservedGeneration is the most recent generation observed by the controller.<br/>
          <br/>
            <i>Format</i>: int64<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>queuedCount</b></td>
        <td>integer</td>
        <td>
          QueuedCount is the total number of pending PipelineRuns across all lanes.<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>runningCount</b></td>
        <td>integer</td>
        <td>
          RunningCount is the total number of running PipelineRuns across all lanes.<br/>
          <br/>
            <i>Format</i>: int32<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PipelineRunQueue.status.conditions[index]
<sup><sup>[↩ Parent](#pipelinerunqueuestatus)</sup></sup>



Condition contains details for one aspect of the current state of this API Resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another.
This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition.
This may be an empty string.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason contains a programmatic identifier indicating the reason for the condition's last transition.
Producers of specific condition types may define expected values and meanings for this field,
and whether the values are considered a guaranteed API.
The value should be a CamelCase string.
This field may not be empty.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
          <br/>
            <i>Enum</i>: True, False, Unknown<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the condition was set based upon.
For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
with respect to the current state of the instance.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### PipelineRunQueue.status.lanes[index]
<sup><sup>[↩ Parent](#pipelinerunqueuestatus)</sup></sup>



LaneStatus reports the derived queue state for a single lane.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          Key is the lane identity, joined values of spec.queueKey labels ("/"-separated; "" for the single lane).<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>queued</b></td>
        <td>[]string</td>
        <td>
          Queued lists names of pending PipelineRuns in admission (FIFO) order.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>running</b></td>
        <td>[]string</td>
        <td>
          Running lists names of PipelineRuns currently occupying lane slots.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>
