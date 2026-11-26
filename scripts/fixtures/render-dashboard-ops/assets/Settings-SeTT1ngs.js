import{r as Zq9}from"./featureflags-F1agsAbC.js";import{n as Qe,y as V}from"./hooks-HooK5x9Q.js";
const rollback=()=>import("./Rollback-R0llBack.js");
var Ld=new Qe(V`
  query statusQuery (
    ${t.map(({index:e})=>`$opt${e}: String!`).join(`\n`)}
  ){
    ${t.map(({index:e})=>`
      alias${e}: status(id: $opt${e}) {
        label
      }
    `).join(``)}
    __typename
  }
`);
export const Settings=()=>Zq9(`okta-app-available`)?rollback:Ld;
