from types import SimpleNamespace
import types


class A:
    def execute(self):
        return 1


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


# SNS attribute-assign method-return under foreign same-leaf.
def use_sns_assign_mr(ba: BoxA, bb: BoxB):
    mrA = SimpleNamespace(k=ba.get()).k
    mrB = SimpleNamespace(k=bb.get()).k
    return mrA.execute() + mrB.run()


def use_types_sns_assign_mr(ba: BoxA, bb: BoxB):
    mrA = types.SimpleNamespace(k=ba.get()).k
    mrB = types.SimpleNamespace(k=bb.get()).k
    return mrA.execute() + mrB.run()


# Local SNS then attr already worked.
def use_sns_local_mr(ba: BoxA, bb: BoxB):
    da = SimpleNamespace(k=ba.get())
    db = SimpleNamespace(k=bb.get())
    return da.k.execute() + db.k.run()


# Inline already worked (receiver peel).
def use_sns_inline_mr(ba: BoxA, bb: BoxB):
    return SimpleNamespace(k=ba.get()).k.execute() + SimpleNamespace(k=bb.get()).k.run()


# Class regression — already worked.
def use_sns_assign_class():
    classA = SimpleNamespace(k=A()).k
    classB = SimpleNamespace(k=B()).k
    return classA.execute() + classB.run()


def use_preserves_b(bb: BoxB):
    mrB = SimpleNamespace(k=bb.get()).k
    return mrB.run()
