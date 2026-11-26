import{n,y as i}from"./hooks-HooK5x9Q.js";var s=new n(i`
  query ownerFeatureFlags($ownerId: String!) {
    owner(ownerId: $ownerId) {
      id
      featureFlags
    }
  }
`),u=(e=null,t)=>!!e?.featureFlags?.some(e=>e===`all`||e===t),d=(e,t)=>u(t,e),m=(e,t)=>t?t.featureFlags.includes(`owner-all`)||t.featureFlags.includes(e):!1,f=e=>m(e,{featureFlags:[]}),p=e=>m(e,null),h=(e,t,n)=>({name:`Experiment Exposed`,experiment_name:e,experiment_group:n?`treatment`:`control`,owner_id:t}),g=()=>({owner:null});export{g as a,p as i,h as n,d as o,f as r,m as t};
