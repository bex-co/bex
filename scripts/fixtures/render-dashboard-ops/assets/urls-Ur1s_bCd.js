import{w as n}from"./git-G1tEnum0.js";
var a=e=>e.startsWith(`/`)&&!e.startsWith(`//`),o=`/billing`,s=e=>`/w/${e}/settings`,c=e=>`${s(e)}#team-members`,d=e=>`/env-group/${e.id}`,f=`/billing/update-plan?plan=professional`,l=e=>`/${n.web}/${e}/*`,u=(e,t,r)=>`/${e.userFacingTypeSlug}/${e.id}/deploys/${t}${r===void 0?``:`?${p(r)}`}`,h=e=>`/${e}/new`,g=`rollback`,y=`https://render.com/docs`,b=e=>y+e,v=`/assets/render-logo.svg`,x=`/graphql`,j=`/env-groups`;
function p(e){let t=new URLSearchParams(e);return t.toString()}
export{a as A,o as B,s as C,c as D,d as E,f as F,l as G,u as H,h as I,g as J,b as K,v as L,x as M,j as N,p as O};
