import{n as Qe,y as V}from"./hooks-HooK5x9Q.js";
var R=new Qe(V`
  query rollbackComparison($serviceId: String!, $deployId: String!) {
    currentLiveDeploy(serviceId: $serviceId) { id }
    deploy(id: $deployId) { id }
    ... on Query { serverPlans { id } }
  }
`);
export{R};
