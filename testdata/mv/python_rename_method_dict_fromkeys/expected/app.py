class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_fromkeys(items: list[A]):
    for a in dict.fromkeys(items):
        a.execute()


def use_fromkeys_b(items: list[B]):
    for b in dict.fromkeys(items):
        b.run()


def use_fromkeys_values(keys: list[str]):
    d = dict.fromkeys(keys, A())
    for a in d.values():
        a.execute()


def use_fromkeys_values_b(keys: list[str]):
    d = dict.fromkeys(keys, B())
    for b in d.values():
        b.run()


def use_fromkeys_value_kw(keys: list[str]):
    d = dict.fromkeys(keys, value=A())
    for a in d.values():
        a.execute()


def use_fromkeys_literal():
    for a in dict.fromkeys([A()]):
        a.execute()
    for b in dict.fromkeys([B()]):
        b.run()


def use_fromkeys_assigned():
    xs = [A()]
    for a in dict.fromkeys(xs):
        a.execute()
    ys = [B()]
    for b in dict.fromkeys(ys):
        b.run()


def use_fromkeys_nested(items: list[A]):
    for a in list(dict.fromkeys(items)):
        a.execute()
