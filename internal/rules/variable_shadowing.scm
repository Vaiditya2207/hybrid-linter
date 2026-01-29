(block
  (short_var_declaration
    left: (expression_list
      (identifier) @shadowed_var))
  (#has-ancestor-with-definition? @shadowed_var)
) @variable_shadowing
