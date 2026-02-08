# vLLM V1 Engine Crash with KV Transfer (P2P NCCL)

## Issue Description
A crash occurs in the vLLM **V1 Engine** on the **Decode** node when running with Disaggregated Serving enabled (KV Cache Transfer via `P2pNcclConnector`). The error manifests as an `IndexError` within the GPU model runner during the input preparation phase.

## Stack Trace
```text
(APIServer pid=1) ERROR 02-08 12:05:52 [async_llm.py:693] AsyncLLM output_handler failed.
(APIServer pid=1) ERROR 02-08 12:05:52 [async_llm.py:693] Traceback (most recent call last):
(APIServer pid=1) ERROR 02-08 12:05:52 [async_llm.py:693]   File "/usr/local/lib/python3.12/dist-packages/vllm/v1/engine/async_llm.py", line 649, in output_handler
(APIServer pid=1) ERROR 02-08 12:05:52 [async_llm.py:693]     outputs = await engine_core.get_output_async()
(APIServer pid=1) ERROR 02-08 12:05:52 [async_llm.py:693]               ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
(APIServer pid=1) ERROR 02-08 12:05:52 [async_llm.py:693]   File "/usr/local/lib/python3.12/dist-packages/vllm/v1/engine/core_client.py", line 894, in get_output_async
(APIServer pid=1) ERROR 02-08 12:05:52 [async_llm.py:693]     raise self._format_exception(outputs) from None
(APIServer pid=1) ERROR 02-08 12:05:52 [async_llm.py:693] vllm.v1.engine.exceptions.EngineDeadError: EngineCore encountered an issue. See stack trace (above) for the root cause.
(EngineCore_DP0 pid=109) Traceback (most recent call last):
(EngineCore_DP0 pid=109)   File "/usr/lib/python3.12/multiprocessing/process.py", line 314, in _bootstrap
(EngineCore_DP0 pid=109)     self.run()
(EngineCore_DP0 pid=109)   File "/usr/lib/python3.12/multiprocessing/process.py", line 108, in run
(EngineCore_DP0 pid=109)     self._target(*self._args, **self._kwargs)
(EngineCore_DP0 pid=109)   File "/usr/local/lib/python3.12/dist-packages/vllm/v1/engine/core.py", line 950, in run_engine_core
(EngineCore_DP0 pid=109)     raise e
(EngineCore_DP0 pid=109)   File "/usr/local/lib/python3.12/dist-packages/vllm/v1/engine/core.py", line 939, in run_engine_core
(EngineCore_DP0 pid=109)     engine_core.run_busy_loop()
(EngineCore_DP0 pid=109)   File "/usr/local/lib/python3.12/dist-packages/vllm/v1/engine/core.py", line 966, in run_busy_loop
(EngineCore_DP0 pid=109)     self._process_engine_step()
(EngineCore_DP0 pid=109)   File "/usr/local/lib/python3.12/dist-packages/vllm/v1/engine/core.py", line 999, in _process_engine_step
(EngineCore_DP0 pid=109)     outputs, model_executed = self.step_fn()
(EngineCore_DP0 pid=109)                               ^^^^^^^^^^^^^^
(EngineCore_DP0 pid=109)   File "/usr/local/lib/python3.12/dist-packages/vllm/v1/engine/core.py", line 490, in step_with_batch_queue
(EngineCore_DP0 pid=109)     exec_model_fut.result()
(EngineCore_DP0 pid=109)   File "/usr/lib/python3.12/concurrent/futures/_base.py", line 449, in result
(APIServer pid=1) INFO:     10.254.14.203:45650 - "POST /v1/completions HTTP/1.1" 200 OK
(EngineCore_DP0 pid=109)     return self.__get_result()
(EngineCore_DP0 pid=109)            ^^^^^^^^^^^^^^^^^^^
(EngineCore_DP0 pid=109)   File "/usr/lib/python3.12/concurrent/futures/_base.py", line 401, in __get_result
(EngineCore_DP0 pid=109)     raise self._exception
(EngineCore_DP0 pid=109)   File "/usr/local/lib/python3.12/dist-packages/vllm/v1/executor/uniproc_executor.py", line 79, in collective_rpc
(EngineCore_DP0 pid=109)     result = run_method(self.driver_worker, method, args, kwargs)
(EngineCore_DP0 pid=109)              ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
(EngineCore_DP0 pid=109)   File "/usr/local/lib/python3.12/dist-packages/vllm/v1/serial_utils.py", line 461, in run_method
(EngineCore_DP0 pid=109)     return func(*args, **kwargs)
(EngineCore_DP0 pid=109)            ^^^^^^^^^^^^^^^^^^^^^
(EngineCore_DP0 pid=109)   File "/usr/local/lib/python3.12/dist-packages/vllm/v1/worker/worker_base.py", line 365, in execute_model
(EngineCore_DP0 pid=109)     return self.worker.execute_model(scheduler_output)
(EngineCore_DP0 pid=109)            ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
(EngineCore_DP0 pid=109)   File "/usr/local/lib/python3.12/dist-packages/torch/utils/_contextlib.py", line 120, in decorate_context
(EngineCore_DP0 pid=109)     return func(*args, **kwargs)
(EngineCore_DP0 pid=109)            ^^^^^^^^^^^^^^^^^^^^^
(APIServer pid=1) INFO:     10.254.14.203:45652 - "POST /v1/completions HTTP/1.1" 200 OK
(EngineCore_DP0 pid=109)   File "/usr/local/lib/python3.12/dist-packages/vllm/v1/worker/gpu_worker.py", line 630, in execute_model
(EngineCore_DP0 pid=109)     output = self.model_runner.execute_model(
(EngineCore_DP0 pid=109)              ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
(EngineCore_DP0 pid=109)   File "/usr/local/lib/python3.12/dist-packages/torch/utils/_contextlib.py", line 120, in decorate_context
(EngineCore_DP0 pid=109)     return func(*args, **kwargs)
(EngineCore_DP0 pid=109)            ^^^^^^^^^^^^^^^^^^^^^
(EngineCore_DP0 pid=109)   File "/usr/local/lib/python3.12/dist-packages/vllm/v1/worker/gpu_model_runner.py", line 3333, in execute_model
(EngineCore_DP0 pid=109)     logits_indices, spec_decode_metadata = self._prepare_inputs(
(EngineCore_DP0 pid=109)                                            ^^^^^^^^^^^^^^^^^^^^^
(EngineCore_DP0 pid=109)   File "/usr/local/lib/python3.12/dist-packages/vllm/v1/worker/gpu_model_runner.py", line 1474, in _prepare_inputs
(EngineCore_DP0 pid=109)     torch.index_select(
(EngineCore_DP0 pid=109) IndexError: index out of range in self
```

## Analysis

### Likely Cause (Hypothesis)
This is a **likely vLLM V1 engine bug** on a KV-transfer path, but not yet proven to be exclusively P2P-specific.

* **Failure location**: `vllm/v1/worker/gpu_model_runner.py`, line 1474, in `_prepare_inputs`.
* **Failure trigger**: `IndexError: index out of range in self` during `torch.index_select`.
* **Observed effect**: `EngineDeadError` and decode-side service instability/restarts.

Why this points to engine internals:
* The exception is thrown inside vLLM core input-preparation logic (not in external proxy code).
* The crash occurs after request scheduling/dispatch enters model execution.
* The symptom is consistent with an internal metadata/shape/index mismatch in decode-time state preparation.

Important scope note:
* The current evidence ties the failure to **V1 + KV transfer/disaggregated serving**.
* It does **not** conclusively prove only `P2pNcclConnector` is affected.
* Treat connector specificity as an open question pending matrix testing.

### Context
* **Engine**: vLLM V1 path
* **Feature**: KV Transfer (Disaggregated Serving)
* **Primary observed connector**: `P2pNcclConnector`
* **Crash role**: Decode node
* **Image in use**: `vllm/vllm-openai:v0.15.0`, suspected to be similar with `vllm/vllm-openai:v0.15.1` (keep restarting, but did not capture the crash back trace)
