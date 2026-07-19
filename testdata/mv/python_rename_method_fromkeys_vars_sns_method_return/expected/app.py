from types import SimpleNamespace


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


class BoxA:
    def __init__(self, a: A):
        self.a = a

    def get(self) -> A:
        return self.a


class BoxB:
    def __init__(self, b: B):
        self.b = b

    def get(self) -> B:
        return self.b


def use_fromkeys(ba: BoxA, bb: BoxB):
    return list(dict.fromkeys([ba.get()]))[0].execute() + list(
        dict.fromkeys([bb.get()])
    )[0].run()


def use_fromkeys_assign(ba: BoxA, bb: BoxB):
    xa = list(dict.fromkeys([ba.get()]))[0]
    xb = list(dict.fromkeys([bb.get()]))[0]
    return xa.execute() + xb.run()


def use_vars_sns(ba: BoxA, bb: BoxB):
    return (
        vars(SimpleNamespace(a=ba.get()))["a"].execute()
        + vars(SimpleNamespace(b=bb.get()))["b"].run()
    )


def use_vars_sns_assign(ba: BoxA, bb: BoxB):
    xa = vars(SimpleNamespace(a=ba.get()))["a"]
    xb = vars(SimpleNamespace(b=bb.get()))["b"]
    return xa.execute() + xb.run()


def use_class():
    return (
        list(dict.fromkeys([A()]))[0].execute()
        + list(dict.fromkeys([B()]))[0].run()
        + vars(SimpleNamespace(a=A()))["a"].execute()
        + vars(SimpleNamespace(b=B()))["b"].run()
    )


def use_preserves_b(bb: BoxB):
    return (
        list(dict.fromkeys([bb.get()]))[0].run()
        + vars(SimpleNamespace(b=bb.get()))["b"].run()
        + vars(SimpleNamespace(b=B()))["b"].run()
    )
