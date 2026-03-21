(
  (expression_statement
    (call_expression
      function: (identifier) @_func)) @unhandled_call
  (#not-match? @_func "^(pr_|print|dev_|EXPORT_|MODULE_|__setup|WARN|BUG|panic|mutex_unlock|spin_unlock|debugfs_|trace_|ASSERT|LOG|lockdep_|spin_lock|mutex_lock)")
)
