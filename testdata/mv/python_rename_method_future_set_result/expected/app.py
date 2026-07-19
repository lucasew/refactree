from concurrent.futures import Future


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_set_result_call():
    fa = Future()
    fb = Future()
    fa.set_result(A())
    fb.set_result(B())
    return fa.result().execute() + fb.result().run()


def use_set_result_assign():
    fa = Future()
    fb = Future()
    fa.set_result(A())
    fb.set_result(B())
    xa = fa.result()
    xb = fb.result()
    return xa.execute() + xb.run()


def use_preserves_b():
    fb = Future()
    fb.set_result(B())
    return fb.result().run()
