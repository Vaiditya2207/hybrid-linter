(go_statement
  (call_expression
    function: (func_literal
      body: (block
        (send_statement
          channel: (identifier) @chan_name)))
  )
) @goroutine_leak

(call_expression
  function: (identifier) @make_func
  arguments: (argument_list
    (identifier) @type
    (int_literal) @capacity)
  (#eq? @make_func "make")
  (#eq? @type "chan")
  (#eq? @capacity "0")
) @unbuffered_chan
