(
  (short_var_declaration
    left: (expression_list
      (identifier) @err)
    right: (expression_list
      (call_expression) @call)) @short_var_declaration
  (#match? @err "^err$|^error$")
  (#not-has-parent? @short_var_declaration if_statement)
)

(
  (assignment_statement
    left: (expression_list
      (identifier) @err)
    right: (expression_list
      (call_expression) @call))
  (#match? @err "^err$|^error$")
)
