import{n,y as i}from"./hooks-HooK5x9Q.js";var m=new n(i`
  query allFrontendFlags($input: String!) {
    allFrontendFlags(input: $input) {
      enabled
      name
    }
  }
`),v=new Map,S=e=>v.get(e),x=e=>S(e)??!1,O=async()=>v,D=async e=>(await O()).get(e)??!1,E=(e,t=`/`)=>async()=>await D(e)?null:t;export{x as o,S as s,E as r,D as n,m as d};
