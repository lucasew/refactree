from dataclasses import dataclass, replace
import dataclasses


@dataclass
class A:
    def execute(self):
        return 1


@dataclass
class B:
    def run(self):
        return 2


class BoxA:
    def __init__(self):
        self.a = A()

    def get(self) -> A:
        return self.a


class BoxB:
    def __init__(self):
        self.b = B()

    def get(self) -> B:
        return self.b


# replace(method-return) under foreign same-leaf.
def use_replace_assign_mr(ba: BoxA, bb: BoxB):
    mrA = replace(ba.get())
    mrB = replace(bb.get())
    return mrA.execute() + mrB.run()


def use_replace_inline_mr(ba: BoxA, bb: BoxB):
    return replace(ba.get()).execute() + replace(bb.get()).run()


def use_dc_replace_assign_mr(ba: BoxA, bb: BoxB):
    mrDA = dataclasses.replace(ba.get())
    mrDB = dataclasses.replace(bb.get())
    return mrDA.execute() + mrDB.run()


def use_replace_walrus_mr(ba: BoxA, bb: BoxB):
    if (mrWA := replace(ba.get())):
        if (mrWB := replace(bb.get())):
            return mrWA.execute() + mrWB.run()
    return 0


# Class regression — already worked for assign; inline now solid too.
def use_replace_assign_class():
    classA = replace(A())
    classB = replace(B())
    return classA.execute() + classB.run()


def use_replace_inline_class():
    return replace(A()).execute() + replace(B()).run()


def use_preserves_b(bb: BoxB):
    mrB = replace(bb.get())
    return mrB.run() + replace(bb.get()).run()
