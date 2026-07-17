class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_setdefault(d: dict[str, A]):
    a = d.setdefault("k")
    a.execute()


def use_setdefault_b(d: dict[str, B]):
    b = d.setdefault("k")
    b.run()


def use_setdefault_default(d: dict[str, A]):
    a = d.setdefault("k", None)
    if a:
        a.execute()


def use_setdefault_default_b(d: dict[str, B]):
    b = d.setdefault("k", None)
    if b:
        b.run()


def use_walrus_setdefault(d: dict[str, A]):
    if (a := d.setdefault("k")):
        a.execute()


def use_walrus_setdefault_b(d: dict[str, B]):
    if (b := d.setdefault("k")):
        b.run()


def use_setdefault_assigned():
    d: dict[str, A] = {}
    a = d.setdefault("k")
    a.execute()
    e: dict[str, B] = {}
    b = e.setdefault("k")
    b.run()
