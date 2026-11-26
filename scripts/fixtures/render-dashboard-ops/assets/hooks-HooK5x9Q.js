var n=class{},y=(e,...t)=>e.join(``);
var a=new n(y`
  mutation updateServer($id: String!, $input: ServerInput! = {name: "a } b"}) {
    renamed: updateServerV2(id: $id, input: $input) @include(if: true) {
      id
      ...serverFields
    }
    deploy(id: $id) {
      id
    }
  }
  ${"fragment serverFields on Server { id }"}
`);
export{n,y,a};
