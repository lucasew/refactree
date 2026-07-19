from contextlib import contextmanager
import contextlib


class A:
    def run(self) -> int:
        return 1

    @staticmethod
    def make():
        return A()

    @classmethod
    def create(cls):
        return cls()

    @staticmethod
    def from_box(ba: "BoxA"):
        return ba.get()

    @classmethod
    def from_box_c(cls, ba: "BoxA"):
        return ba.get()

    @staticmethod
    def from_box_assign(ba: "BoxA"):
        cxa = ba.get()
        return cxa


class B:
    def run(self) -> int:
        return 2

    @staticmethod
    def make():
        return B()

    @classmethod
    def create(cls):
        return cls()

    @staticmethod
    def from_box(bb: "BoxB"):
        return bb.get()

    @classmethod
    def from_box_c(cls, bb: "BoxB"):
        return bb.get()

    @staticmethod
    def from_box_assign(bb: "BoxB"):
        cxb = bb.get()
        return cxb


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


# --- Class @staticmethod/@classmethod factories (already solid). ---
def use_class_static() -> int:
    return A.make().run() + B.make().run()


def use_class_classmethod() -> int:
    return A.create().run() + B.create().run()


def use_class_static_assign() -> int:
    csa = A.make()
    csb = B.make()
    return csa.run() + csb.run()


# --- Method-return class factories (were UNDER). ---
def use_mr_static(ba: BoxA, bb: BoxB) -> int:
    return A.from_box(ba).run() + B.from_box(bb).run()


def use_mr_classmethod(ba: BoxA, bb: BoxB) -> int:
    return A.from_box_c(ba).run() + B.from_box_c(bb).run()


def use_mr_static_assign(ba: BoxA, bb: BoxB) -> int:
    msa = A.from_box(ba)
    msb = B.from_box(bb)
    return msa.run() + msb.run()


def use_mr_static_body_assign(ba: BoxA, bb: BoxB) -> int:
    return A.from_box_assign(ba).run() + B.from_box_assign(bb).run()


# --- Class contextmanager factories (already solid). ---
@contextmanager
def make_cca():
    yield A()


@contextmanager
def make_ccb():
    yield B()


@contextlib.contextmanager
def make_cca2():
    cya = A()
    yield cya


# --- Method-return contextmanager factories (were UNDER). ---
@contextmanager
def make_mca(ba: BoxA):
    yield ba.get()


@contextmanager
def make_mcb(bb: BoxB):
    yield bb.get()


@contextmanager
def make_mcaa(ba: BoxA):
    mya = ba.get()
    yield mya


@contextmanager
def make_mcab(bb: BoxB):
    myb = bb.get()
    yield myb


def use_class_cm() -> int:
    with make_cca() as cxa:
        with make_ccb() as cxb:
            return cxa.run() + cxb.run()


def use_class_cm_assign_yield() -> int:
    with make_cca2() as cya:
        return cya.run()


def use_mr_cm(ba: BoxA, bb: BoxB) -> int:
    with make_mca(ba) as mxa:
        with make_mcb(bb) as mxb:
            return mxa.run() + mxb.run()


def use_mr_cm_assign_yield(ba: BoxA, bb: BoxB) -> int:
    with make_mcaa(ba) as mya:
        with make_mcab(bb) as myb:
            return mya.run() + myb.run()


# --- Nested free-var generator yields: Class solid / MR was UNDER. ---
def use_nested_gen(ba: BoxA, bb: BoxB) -> int:
    def gen_cna():
        yield A()

    def gen_cnb():
        yield B()

    def gen_mna():
        yield ba.get()

    def gen_mnb():
        yield bb.get()

    def gen_maa():
        nxa = ba.get()
        yield nxa

    def gen_mab():
        nxb = bb.get()
        yield nxb

    return (
        next(gen_cna()).run()
        + next(gen_cnb()).run()
        + next(gen_mna()).run()
        + next(gen_mnb()).run()
        + next(gen_maa()).run()
        + next(gen_mab()).run()
    )


# --- Top-level param generators (already solid; regression). ---
def gen_mpa(ba: BoxA):
    yield ba.get()


def gen_mpb(bb: BoxB):
    yield bb.get()


def use_param_gen(ba: BoxA, bb: BoxB) -> int:
    return next(gen_mpa(ba)).run() + next(gen_mpb(bb)).run()


def use_preserves_b(ba: BoxA, bb: BoxB) -> int:
    return (
        B.make().run()
        + B.from_box(bb).run()
        + next(gen_mpb(bb)).run()
    )
