class A:
    def execute(self):
        return self.run_helper()

    def run_helper(self):
        return 1


class B:
    def run(self):
        return 2


def use_a(a: A):
    return a.execute()


def use_b(b: B):
    return b.run()
