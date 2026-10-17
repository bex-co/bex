%module qa_swig
%inline %{
int weighted_sum(int a,int b) { return a*2+b*3; }
const char *dessert(void) { return "Crème brûlée & 李"; }
%}
