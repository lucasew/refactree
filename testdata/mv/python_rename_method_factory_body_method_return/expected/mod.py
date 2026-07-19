class A:
    def execute(self) -> int:
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


# --- Class body-only factories (already solid). ---
def make_ca():
    return A()


def make_cb():
    return B()


def make_caa():
    cxa = A()
    return cxa


def make_cab():
    cxb = B()
    return cxb


# --- Method-return body-only factories (were UNDER). ---
def make_ma(ba: BoxA):
    return ba.get()


def make_mb(bb: BoxB):
    return bb.get()


def make_maa(ba: BoxA):
    mxa = ba.get()
    return mxa


def make_mab(bb: BoxB):
    mxb = bb.get()
    return mxb


# --- Annotated factories (already solid both sides; regression). ---
def make_ca_ann() -> A:
    return A()


def make_cb_ann() -> B:
    return B()


def make_ma_ann(ba: BoxA) -> A:
    return ba.get()


def make_mb_ann(bb: BoxB) -> B:
    return bb.get()


def use_class_body() -> int:
    return make_ca().execute() + make_cb().run()


def use_mr_body(ba: BoxA, bb: BoxB) -> int:
    return make_ma(ba).execute() + make_mb(bb).run()


def use_class_body_assign() -> int:
    return make_caa().execute() + make_cab().run()


def use_mr_body_assign(ba: BoxA, bb: BoxB) -> int:
    return make_maa(ba).execute() + make_mab(bb).run()


def use_class_call_assign() -> int:
    cya = make_ca()
    cyb = make_cb()
    return cya.execute() + cyb.run()


def use_mr_call_assign(ba: BoxA, bb: BoxB) -> int:
    mya = make_ma(ba)
    myb = make_mb(bb)
    return mya.execute() + myb.run()


def use_class_nested() -> int:
    def make_cna():
        return A()

    def make_cnb():
        return B()

    return make_cna().execute() + make_cnb().run()


def use_mr_nested(ba: BoxA, bb: BoxB) -> int:
    def make_mna():
        return ba.get()

    def make_mnb():
        return bb.get()

    return make_mna().execute() + make_mnb().run()


def use_class_lambda() -> int:
    cfa = lambda: A()
    cfb = lambda: B()
    return cfa().execute() + cfb().run()


def use_mr_lambda(ba: BoxA, bb: BoxB) -> int:
    mfa = lambda: ba.get()
    mfb = lambda: bb.get()
    return mfa().execute() + mfb().run()


def use_class_ann() -> int:
    return make_ca_ann().execute() + make_cb_ann().run()


def use_mr_ann(ba: BoxA, bb: BoxB) -> int:
    return make_ma_ann(ba).execute() + make_mb_ann(bb).run()


def preserves_b(bb: BoxB) -> int:
    pfb = lambda: bb.get()
    return make_mb(bb).run() + make_mab(bb).run() + make_mb_ann(bb).run() + pfb().run()
