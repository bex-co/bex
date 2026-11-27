import{n as Qe,y as V}from"./hooks-HooK5x9Q.js";import{a as Ox,i as kx,r as jx,t as Mx}from"./featureflags-F1agsAbC.js";import{r as Co}from"./frontendFlag-Fr0ntF1g.js";import{B as Ri,C as Ws,G as Sv,I as Na}from"./urls-Ur1s_bCd.js";import{w as al}from"./git-G1tEnum0.js";
var Mu=new Qe(V`
  query server($id: String!) {
    server(id: $id) {
      id
      name
    }
  }
`),Nu=new Qe(V`
  subscription serviceEvents($serviceId: String!) {
    serviceEvents(serviceId: $serviceId) {
      id
    }
  }
`);
const lazy=()=>import("./Settings-SeTT1ngs.js"),deps=["assets/Billing-B1ll-ng0.js"],routes=[{path:`/sandboxes`,loader:Co(`sandbox_shell`)}];
var Kr=`/env-groups`,Tb={app:Ap,routes:[{Component:Ap,children:[{path:Sv(`:serverId`),Component:Ap,children:Ch},{path:Ri,Component:Ap},{path:Kr,Component:Ap},{path:`/w/:ownerId/billing/update-plan-preview`,Component:Ap},{path:`/env-group/:id`,Component:Ap}]}]},Ch=[{path:`*`,Component:Ap},{path:`events`,Component:Ap},{path:`metrics/:period`,Component:Ap}],Rt=Bz([{element:null,children:[...Tb.routes,...routes]}]),Mn=[{title:`Web Service`,path:Na(al.web)},{title:`Static Site`,path:Na(al.static)}],Nv=e=>[{title:`Settings`,href:Ws(e),hrefPatterns:[`/w/:id/settings/*`,Ws(`:id`)]}];
function Inner(){let t=`x`;return Qe({path:t,element:null})}
function Card(e){let t=jx(`postgres-parameter-overrides`),n=Mx(`share-postgres`,e.owner),r=kx(`skip-banner`);return t&&n&&r}
// Prose that mentions a `next` query param is not an operation.
const note=`"${e}" is part of the routes and shouldn't be used as a \`next\` query param`;
export{Card,lazy,deps,routes,note,Rt,Mn,Nv,Inner};
