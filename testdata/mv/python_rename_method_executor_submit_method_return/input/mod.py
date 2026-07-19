from concurrent.futures import ProcessPoolExecutor, ThreadPoolExecutor


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    a: A

    def __init__(self, a: A) -> None:
        self.a = a

    def get(self) -> A:
        return self.a


class BoxB:
    b: B

    def __init__(self, b: B) -> None:
        self.b = b

    def get(self) -> B:
        return self.b


# --- Class regressions: submit(lambda: Class()).result (already solid). ---
def use_class_inline() -> int:
    with ThreadPoolExecutor() as ex:
        return (
            ex.submit(lambda: A()).result().run()
            + ex.submit(lambda: B()).result().run()
        )


def use_class_result_assign() -> int:
    with ThreadPoolExecutor() as ex:
        xa = ex.submit(lambda: A()).result()
        xb = ex.submit(lambda: B()).result()
        return xa.run() + xb.run()


def use_class_process() -> int:
    with ProcessPoolExecutor() as ex:
        return (
            ex.submit(lambda: A()).result().run()
            + ex.submit(lambda: B()).result().run()
        )


def use_class_timeout() -> int:
    with ThreadPoolExecutor() as ex:
        return (
            ex.submit(lambda: A()).result(timeout=1).run()
            + ex.submit(lambda: B()).result(timeout=1).run()
        )


# --- Method-return under foreign same-leaf. ---
def use_mr_inline(ba: BoxA, bb: BoxB) -> int:
    with ThreadPoolExecutor() as ex:
        return (
            ex.submit(lambda: ba.get()).result().run()
            + ex.submit(lambda: bb.get()).result().run()
        )


def use_mr_result_assign(ba: BoxA, bb: BoxB) -> int:
    with ThreadPoolExecutor() as ex:
        xa = ex.submit(lambda: ba.get()).result()
        xb = ex.submit(lambda: bb.get()).result()
        return xa.run() + xb.run()


def use_mr_process(ba: BoxA, bb: BoxB) -> int:
    with ProcessPoolExecutor() as ex:
        return (
            ex.submit(lambda: ba.get()).result().run()
            + ex.submit(lambda: bb.get()).result().run()
        )


def use_mr_timeout(ba: BoxA, bb: BoxB) -> int:
    with ThreadPoolExecutor() as ex:
        return (
            ex.submit(lambda: ba.get()).result(timeout=1).run()
            + ex.submit(lambda: bb.get()).result(timeout=1).run()
        )


def use_preserves_b(bb: BoxB) -> int:
    with ThreadPoolExecutor() as ex:
        return (
            ex.submit(lambda: bb.get()).result().run()
            + ex.submit(lambda: B()).result().run()
            + ex.submit(lambda: bb.get()).result(timeout=1).run()
        )
