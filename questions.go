package processing

import (
	vocab "github.com/go-ap/activitypub"
)

// QuestionActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-questions
//
// The Questions use case primarily deals with representing inquiries of any type.
// See 5.4 Representing Questions for more information: https://www.w3.org/TR/activitystreams-vocabulary/#questions
func (p *P) QuestionActivity(q *vocab.Question) (*vocab.Question, error) {
	// NOTE(marius): this behaviour is not in accordance to the spec.
	// Saving the question items as individual objects makes sense to me,
	// but it's not mention in either of the documents we're basing this implementation on.
	// I can't think of any reason why saving them might be a _bad_ idea, so for the moment I'll leave it like this.
	saveAnswersFn := p.saveQuestionAnswers(q)
	if q.AnyOf != nil {
		return q, vocab.OnItem(q.AnyOf, saveAnswersFn)
	}
	if q.OneOf != nil {
		return q, vocab.OnItem(q.OneOf, saveAnswersFn)
	}
	return q, nil
}

func (p *P) saveQuestionAnswers(q *vocab.Question) func(vocab.Item) error {
	return func(ans vocab.Item) error {
		var err error
		if iri := ans.GetLink(); len(iri) == 0 {
			if err = SetIDIfMissing(ans, q, p.createIDFn); err != nil {
				return err
			}
		}

		// NOTE(marius): As we mentioned in the comment in the calling method,
		// this is an assumption on our part that we want to apply the same logic for
		// an Object Create on the answers to our Question activity.
		_ = p.updateObjectForCreate(ans, q)

		ans, err = p.s.Save(ans)
		if err != nil {
			return err
		}
		return nil
	}
}
