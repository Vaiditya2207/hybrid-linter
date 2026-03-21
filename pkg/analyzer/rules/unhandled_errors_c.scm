(
  (expression_statement
    (call_expression
      function: (identifier) @_func)) @unhandled_call
  (#not-match? @_func "^(pr_|print|dev_|EXPORT_|MODULE_|__setup|late_init|core_init|postcore_init|arch_init|subsys_init|fs_init|device_init|pure_init|module_init|module_exit|WARN|BUG|panic|mutex_|spin_|raw_spin_|read_lock|read_unlock|write_lock|write_unlock|rcu_|debugfs_|trace_|lockdep_|smp_|cpu_|kfree|kmem_cache_free|free_page|vfree|put_device|put_task_struct|wait_event|wake_up)")
)
