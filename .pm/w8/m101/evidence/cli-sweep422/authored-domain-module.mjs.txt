export function summarize(rows) {
  return {labels:rows.map(row=>row.label),total:rows.reduce((n,row)=>n+row.quantity,0),sorted:rows.map(row=>row.quantity).sort((a,b)=>a-b)};
}
