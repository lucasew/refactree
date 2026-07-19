class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_get(d: dict[str, A]):
    a = d.get("k")
    a.execute()


def use_get_b(d: dict[str, B]):
    b = d.get("k")
    b.run()


def use_get_default(d: dict[str, A]):
    a = d.get("k", None)
    if a:
        a.execute()


def use_get_default_b(d: dict[str, B]):
    b = d.get("k", None)
    if b:
        b.run()


def use_walrus_get(d: dict[str, A]):
    if (a := d.get("k")):
        a.execute()


def use_walrus_get_b(d: dict[str, B]):
    if (b := d.get("k")):
        b.run()


def use_get_assigned():
    d: dict[str, A] = {}
    a = d.get("k")
    a.execute()
    e: dict[str, B] = {}
    b = e.get("k")
    b.run()
